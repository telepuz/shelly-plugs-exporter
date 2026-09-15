# Agent guidelines

## HTTP endpoints

- `GET /metrics` - Prometheus metrics
- `GET /healthz` - liveness probe, always returns `200 OK` with body `ok`

## Scope rules

- New files: `cmd/`, `internal/` only - never put logic in root
- Collector lives in `internal/collector/` - no Shelly HTTP calls there, only metric assembly
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

HTTP client must set timeout from `SCRAPE_TIMEOUT` ENV var. Use `context` from caller.
Shelly Gen3 uses HTTP Digest auth. If device has `username`+`password` - implement digest auth.
Use `github.com/icholy/digest` package or manual digest challenge-response.

## Prometheus conventions

- Gauge for all current readings (power, voltage, energy counters from device)
- Metric descriptions in `var` block at package level, not inline
- `prometheus.MustRegister` only at startup, never in hot path
- Use `prometheus.NewDesc` with `constLabels` for device identity

## Error handling

- Device unreachable: set `shelly_up{...} = 0`, skip other metrics for that device (do not return error to collector)
- Missing `DEVICES` ENV or JSON parse error: fatal at startup
- HTTP 4xx/5xx from device: log warning, set `shelly_up = 0`

## Concurrency

- Collect all devices in parallel during each scrape
- Use `sync.WaitGroup` + channel or `golang.org/x/sync/errgroup`
- Each device gets its own HTTP client (or shared client with per-request context)

## Logging

Use `log/slog` with structured fields:

```go
slog.Warn("device unreachable", "device", dev.Name, "address", dev.Address, "err", err)
```

No `fmt.Printf` for logging.

## What NOT to do

- Do not cache metric values between scrapes - always poll fresh
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
