# Agent guidelines

## HTTP endpoints

- `GET /metrics` - Prometheus metrics
- `GET /healthz` - liveness probe, always returns `200 OK` with body `ok`

## Scope rules

- New files: `cmd/`, `internal/` only - never put logic in root
- Collector lives in `internal/collector/` - no Shelly HTTP calls there, only metric assembly and cache reads
- Shelly API calls live in `internal/shelly/` - pure HTTP, no prometheus types there
- Config loading lives in `internal/config/` - no business logic

## Shelly RPC API

Base URL: `http://<address>/rpc`

All endpoints accept GET with digest auth if credentials set. Response is JSON.

Only two endpoints needed for PlugS Gen3:

**`GET /rpc/Switch.GetStatus?id=0`** - all power metrics:
```json
{
  "output": true,
  "apower": 106.3,
  "voltage": 230.3,
  "freq": 50.0,
  "current": 0.667,
  "aenergy": { "total": 267.813 },
  "ret_aenergy": { "total": 0.0 },
  "temperature": { "tC": 43.3 }
}
```

**`GET /rpc/Sys.GetStatus`** - system info:
```json
{ "uptime": 12345 }
```

Note: EM.GetStatus, EMData.GetStatus, Temperature.GetStatus are for Shelly EM Pro - NOT for PlugS Gen3.

HTTP client must set timeout from `SCRAPE_TIMEOUT` ENV var (applies per attempt, not total).
Shelly Gen3 uses HTTP Digest auth. If device has `username`+`password` - implement digest auth.
Use `github.com/icholy/digest` package.

## Caching and background polling

**Key invariant**: Collect() never makes HTTP calls. It reads from in-memory cache only.

Cache is populated by background goroutines started via `Collector.Start(ctx context.Context)`.
One goroutine per device. Each goroutine:
1. Polls immediately at startup
2. Then polls on `time.Ticker` every `POLL_INTERVAL`
3. Writes result under `sync.Mutex`

Cache struct per device:
```go
type deviceCache struct {
    mu           sync.RWMutex
    up           float64
    switchStatus shelly.SwitchStatus  // zero value when up=0
    sysStatus    shelly.SysStatus     // zero value when up=0
}
```

When device unreachable: set up=0, leave switchStatus/sysStatus as zero values.
When device reachable: set up=1, store fetched values.

**Collect() behavior when up=0**: emit ALL metrics with value 0 (not skip them).
This differs from old behavior (which skipped metrics when up=0).

## Retry policy

Retry only on network errors (connection refused, timeout, DNS failure).
Do NOT retry on HTTP 4xx/5xx - the device responded, that is definitive.

3 attempts total (1 initial + 2 retries).
Fixed 500ms delay between attempts. Respect context cancellation in delay.

Network error detection: `errors.As(err, &*url.Error{})` where `Temporary()` or `Timeout()` is true,
or the inner error is `*net.OpError`. Also retry on `io.EOF` and `io.ErrUnexpectedEOF`.

Retry logic belongs in `internal/shelly/client.go`, wrapping each API call.

## Prometheus conventions

- Gauge for all current readings (power, voltage, energy counters from device)
- Metric descriptions in `var` block at package level, not inline
- `prometheus.MustRegister` only at startup, never in hot path
- Use `prometheus.NewDesc` with `constLabels` for device identity

## Error handling

- Device unreachable or network error after all retries: set `shelly_up{...} = 0`, all other metrics = 0
- HTTP 4xx/5xx from device (no retry): set `shelly_up = 0`, all other metrics = 0
- Missing `DEVICES` ENV or JSON parse error: fatal at startup
- Log warning on each failed poll with device name, address, error

## Concurrency

- Background polling: one goroutine per device, started by `Collector.Start(ctx)`
- Each goroutine owns its slice of the state array - no contention between goroutines on write
- Collect() reads all device states under RLock per device

## Logging

Use `log/slog` with structured fields:

```go
slog.Warn("device poll failed", "device", dev.Name, "address", dev.Address, "err", err)
```

No `fmt.Printf` for logging.

## What NOT to do

- Do not poll devices in Collect() - all HTTP must happen in background goroutines
- Do not use `init()` functions
- Do not use global variables for state (only for metric descriptors)
- Do not swallow errors silently - log or propagate
- Do not add ENV vars or config not described in CLAUDE.md without asking
- Do not use CLI flags or config files - ENV vars only

## Testing approach

Mock Shelly API with `net/http/httptest`:

```go
srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    switch r.URL.Path {
    case "/rpc/Switch.GetStatus":
        json.NewEncoder(w).Encode(SwitchStatus{Output: true})
    }
}))
defer srv.Close()
```

Test collector output with `github.com/prometheus/client_golang/prometheus/testutil`.

For cache tests: call `Start()`, wait for initial poll to complete (use a sync mechanism or short sleep),
then check metrics. For unavailable-then-available: swap server handler between polls.

For retry tests: use a counter in the handler to fail first N attempts, succeed on N+1.

## Key behavioral changes from old implementation

Old: when up=0, only `shelly_up` metric emitted.
New: when up=0, ALL metrics emitted with value 0 (shelly_up=0, shelly_active_power_watts=0, etc.).

Old: Collect() blocks on HTTP.
New: Collect() is instant - reads cache. HTTP happens in background.

Old: no retries.
New: 3 attempts on network errors with 500ms delay.
