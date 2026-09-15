# shelly-plugs-exporter

Prometheus exporter for Shelly PlugS Gen3 smart plugs. Written in Go.

## Project overview

One exporter instance polls N devices via HTTP (Shelly JSON-RPC API).
All configuration via ENV vars - no config files, no CLI flags.
Exporter exposes `/metrics` for Prometheus and `/healthz` for Kubernetes liveness probe.

## Architecture

```
ENV vars (DEVICES, LISTEN_ADDRESS, SCRAPE_TIMEOUT)
     |
main.go -> Collector (prometheus.Collector)
              |
              +-- per device: HTTP GET /rpc/Switch.GetStatus?id=0
              +-- per device: HTTP GET /rpc/Sys.GetStatus
```

Concurrent polling - one goroutine per device per scrape cycle (via `errgroup` or `sync.WaitGroup`).

## Module name

```
github.com/telepuz/shelly-plugs-exporter
```

## Key packages

- `prometheus/client_golang` - metrics exposure
- `net/http` - HTTP client for Shelly API
- `encoding/json` - DEVICES JSON parsing

## Directory layout

```
cmd/exporter/main.go      - entrypoint, ENV loading, HTTP server
internal/collector/       - prometheus.Collector implementation
internal/shelly/          - Shelly RPC client, response types
internal/config/          - config struct and ENV loader
Dockerfile
```

## Metrics exposed

All metric names prefixed with `shelly_`.

Source: `Switch.GetStatus?id=0` actual response fields.

| Metric | Type | JSON field |
|--------|------|-----------|
| `shelly_up` | gauge | 1 if device reachable, 0 otherwise |
| `shelly_switch_output` | gauge | `.output` (bool -> 0/1) |
| `shelly_active_power_watts` | gauge | `.apower` |
| `shelly_voltage_volts` | gauge | `.voltage` |
| `shelly_frequency_hz` | gauge | `.freq` |
| `shelly_current_amperes` | gauge | `.current` |
| `shelly_energy_total_wh` | gauge | `.aenergy.total` |
| `shelly_returned_energy_total_wh` | gauge | `.ret_aenergy.total` |
| `shelly_temperature_celsius` | gauge | `.temperature.tC` |
| `shelly_sys_uptime_seconds` | gauge | `Sys.GetStatus` `.uptime` |

Labels on all metrics: `device` (name from config), `address` (host).

## Configuration - ENV vars

| Var | Default | Required | Description |
|-----|---------|----------|-------------|
| `DEVICES` | - | yes | JSON array of devices |
| `LISTEN_ADDRESS` | `:9924` | no | HTTP listen address |
| `SCRAPE_TIMEOUT` | `10s` | no | Per-device HTTP timeout |

`DEVICES` example:
```json
[{"name":"office","address":"192.168.1.100","username":"admin","password":"secret"},{"name":"kitchen","address":"192.168.1.101","username":"admin","password":"secret"}]
```

`username`/`password` optional per device. Auth method: HTTP Digest (Shelly Gen3 standard).

## Build and run

```bash
go build -o bin/exporter ./cmd/exporter/
DEVICES='[{"name":"plug1","address":"192.168.1.100"}]' ./bin/exporter

# Docker
docker build -t shelly-plugs-exporter .
docker run -e DEVICES='[{"name":"plug1","address":"192.168.1.100"}]' -p 9924:9924 shelly-plugs-exporter
```

## Testing

Unit tests use `httptest.Server` to mock Shelly RPC endpoints - no real devices needed.

```bash
go test ./...
go vet ./...
```

## Code style

- No comments except non-obvious WHY
- Error wrap with `fmt.Errorf("...: %w", err)`
- Structured logging via `log/slog`
- Context propagation for HTTP requests (respects scrape timeout)
- No global state - pass config/deps explicitly
