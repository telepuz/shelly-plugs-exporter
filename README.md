# Shelly PlugS Exporter

Prometheus exporter for Shelly PlugS Gen3 smart plugs.

## Quick start

```bash
docker run -d \
  -e DEVICES='[{"name":"office","address":"192.168.1.100","username":"admin","password":"secret"}]' \
  -e LISTEN_ADDRESS=":9924" \
  -e SCRAPE_TIMEOUT="30s" \
  -p 9924:9924 \
  t7k312/shelly-plugs-exporter:latest
```

## Environment variables

| Variable | Default | Required | Description |
|---|---|---|---|
| `DEVICES` | - | yes | JSON array of device definitions (see format below) |
| `LISTEN_ADDRESS` | `:9924` | no | Address and port for the HTTP server |
| `SCRAPE_TIMEOUT` | `30s` | no | Per-device HTTP request timeout (Go duration format) |

## DEVICES format

`DEVICES` is a JSON array of objects. Each object describes one Shelly PlugS Gen3 device.

```json
[
  {
    "name": "office",
    "address": "192.168.1.100",
    "username": "admin",
    "password": "changeme"
  },
  {
    "name": "kitchen",
    "address": "192.168.1.101"
  }
]
```

Fields:

| Field | Required | Description |
|---|---|---|
| `name` | yes | Human-readable label used as the `device` Prometheus label |
| `address` | yes | IP address or hostname of the device |
| `username` | no | HTTP Digest auth username |
| `password` | no | HTTP Digest auth password |

## Prometheus scrape config

```yaml
scrape_configs:
  - job_name: shelly
    static_configs:
      - targets:
          - localhost:9924
```

## Metrics

All metrics carry `device` and `address` labels.

| Metric | Type | Description |
|---|---|---|
| `shelly_up` | gauge | 1 if device is reachable, 0 otherwise |
| `shelly_switch_output` | gauge | Switch state (1=on, 0=off) |
| `shelly_active_power_watts` | gauge | Active power in watts |
| `shelly_voltage_volts` | gauge | Measured voltage in volts |
| `shelly_frequency_hz` | gauge | AC frequency in hertz |
| `shelly_current_amperes` | gauge | Measured current in amperes |
| `shelly_energy_total_wh` | gauge | Total energy consumed in watt-hours |
| `shelly_returned_energy_total_wh` | gauge | Total energy returned in watt-hours |
| `shelly_temperature_celsius` | gauge | Device temperature in Celsius |
| `shelly_sys_uptime_seconds` | gauge | System uptime in seconds |

## Kubernetes deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: shelly-plugs-exporter
  labels:
    app: shelly-plugs-exporter
spec:
  replicas: 1
  selector:
    matchLabels:
      app: shelly-plugs-exporter
  template:
    metadata:
      labels:
        app: shelly-plugs-exporter
    spec:
      containers:
        - name: exporter
          image: your-dockerhub-user/shelly-plugs-exporter:latest
          ports:
            - containerPort: 9924
          env:
            - name: DEVICES
              value: '[{"name":"office","address":"192.168.1.100","username":"admin","password":"changeme"}]'
            - name: LISTEN_ADDRESS
              value: ":9924"
            - name: SCRAPE_TIMEOUT
              value: "30s"
          resources:
            requests:
              cpu: 10m
              memory: 32Mi
            limits:
              cpu: 100m
              memory: 128Mi
          readinessProbe:
            httpGet:
              path: /healthz
              port: 9924
            initialDelaySeconds: 5
            periodSeconds: 10
          livenessProbe:
            httpGet:
              path: /healthz
              port: 9924
            initialDelaySeconds: 10
            periodSeconds: 30
---
apiVersion: v1
kind: Service
metadata:
  name: shelly-plugs-exporter
  labels:
    app: shelly-plugs-exporter
spec:
  selector:
    app: shelly-plugs-exporter
  ports:
    - name: metrics
      port: 9924
      targetPort: 9924
```

## Integration tests

Integration tests require a real Shelly PlugS Gen3 device. Pass credentials via environment variables - never hardcode them.

```bash
SHELLY_TEST_ADDRESS=192.168.0.106 \
SHELLY_TEST_USERNAME=admin \
SHELLY_TEST_PASSWORD=<password> \
go test -tags integration ./internal/shelly/ -v
```

| Variable | Default | Description |
|---|---|---|
| `SHELLY_TEST_ADDRESS` | `192.168.0.106` | Device IP address |
| `SHELLY_TEST_USERNAME` | `admin` | Auth username |
| `SHELLY_TEST_PASSWORD` | - | Auth password (required) |
