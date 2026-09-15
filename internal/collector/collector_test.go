package collector

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/telepuz/shelly-plugs-exporter/internal/config"
)

const switchStatusJSON = `{"output":true,"apower":106.3,"voltage":230.3,"freq":50.0,"current":0.667,"aenergy":{"total":267.813},"ret_aenergy":{"total":0.0},"temperature":{"tC":43.3}}`
const sysStatusJSON = `{"uptime":12345}`

// newShellyServer returns a running httptest.Server that responds like a Shelly PlugS Gen3.
// switchCode and sysCode control the HTTP status codes for each endpoint.
func newShellyServer(t *testing.T, switchCode int, switchBody string, sysCode int, sysBody string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/rpc/Switch.GetStatus", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(switchCode)
		if switchBody != "" {
			w.Write([]byte(switchBody))
		}
	})
	mux.HandleFunc("/rpc/Sys.GetStatus", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(sysCode)
		if sysBody != "" {
			w.Write([]byte(sysBody))
		}
	})
	return httptest.NewServer(mux)
}

// hostPort strips the "http://" scheme from a URL returned by httptest.Server.
func hostPort(url string) string {
	return strings.TrimPrefix(url, "http://")
}

// newRegistry creates a fresh Prometheus registry with only the given collector registered.
func newRegistry(t *testing.T, c prometheus.Collector) *prometheus.Registry {
	t.Helper()
	reg := prometheus.NewRegistry()
	if err := reg.Register(c); err != nil {
		t.Fatalf("failed to register collector: %v", err)
	}
	return reg
}

// assertMetricValue asserts that a gauge metric with the given name has the expected value
// for the given label values.
func assertMetricValue(t *testing.T, reg *prometheus.Registry, metricName string, wantValue float64, labelValues ...string) {
	t.Helper()
	mfs, err := reg.Gather()
	if err != nil {
		t.Fatalf("gather failed: %v", err)
	}
	for _, mf := range mfs {
		if mf.GetName() != metricName {
			continue
		}
		for _, m := range mf.GetMetric() {
			labels := m.GetLabel()
			if len(labels) != len(labelValues)/2 {
				continue
			}
			match := true
			for i := 0; i+1 < len(labelValues); i += 2 {
				key, val := labelValues[i], labelValues[i+1]
				found := false
				for _, lp := range labels {
					if lp.GetName() == key && lp.GetValue() == val {
						found = true
						break
					}
				}
				if !found {
					match = false
					break
				}
			}
			if match {
				got := m.GetGauge().GetValue()
				if got != wantValue {
					t.Errorf("metric %s %v: got %v, want %v", metricName, labelValues, got, wantValue)
				}
				return
			}
		}
	}
	t.Errorf("metric %s with labels %v not found", metricName, labelValues)
}

// allMetricNames returns the set of metric family names present in a registry gather result.
func allMetricNames(t *testing.T, reg *prometheus.Registry) map[string]struct{} {
	t.Helper()
	mfs, err := reg.Gather()
	if err != nil {
		t.Fatalf("gather failed: %v", err)
	}
	names := make(map[string]struct{}, len(mfs))
	for _, mf := range mfs {
		names[mf.GetName()] = struct{}{}
	}
	return names
}

// hasMetricForDevice checks whether the gathered metrics contain at least one sample
// for the given metric name and device label.
func hasMetricForDevice(t *testing.T, reg *prometheus.Registry, metricName, deviceName string) bool {
	t.Helper()
	mfs, err := reg.Gather()
	if err != nil {
		t.Fatalf("gather failed: %v", err)
	}
	for _, mf := range mfs {
		if mf.GetName() != metricName {
			continue
		}
		for _, m := range mf.GetMetric() {
			for _, lp := range m.GetLabel() {
				if lp.GetName() == "device" && lp.GetValue() == deviceName {
					return true
				}
			}
		}
	}
	return false
}

func newTestConfig(devices []config.Device, timeout time.Duration) *config.Config {
	return &config.Config{
		Devices:       devices,
		ScrapeTimeout: timeout,
		PollInterval:  time.Minute,
	}
}

// TestCollector_HappyPath verifies that all 10 metrics are emitted with correct values
// when both Shelly API endpoints return valid data.
func TestCollector_HappyPath(t *testing.T) {
	srv := newShellyServer(t, http.StatusOK, switchStatusJSON, http.StatusOK, sysStatusJSON)
	defer srv.Close()

	addr := hostPort(srv.URL)
	cfg := newTestConfig([]config.Device{{Name: "plug1", Address: addr}}, 5*time.Second)
	c := New(cfg)
	c.PollAll(context.Background())
	reg := newRegistry(t, c)

	// Verify total metric count: 10 metric families, each with 1 sample.
	count := testutil.CollectAndCount(c)
	if count != 10 {
		t.Errorf("expected 10 metrics, got %d", count)
	}

	assertMetricValue(t, reg, "shelly_up", 1, "device", "plug1", "address", addr)
	assertMetricValue(t, reg, "shelly_switch_output", 1, "device", "plug1", "address", addr)
	assertMetricValue(t, reg, "shelly_active_power_watts", 106.3, "device", "plug1", "address", addr)
	assertMetricValue(t, reg, "shelly_voltage_volts", 230.3, "device", "plug1", "address", addr)
	assertMetricValue(t, reg, "shelly_frequency_hz", 50.0, "device", "plug1", "address", addr)
	assertMetricValue(t, reg, "shelly_current_amperes", 0.667, "device", "plug1", "address", addr)
	assertMetricValue(t, reg, "shelly_energy_total_wh", 267.813, "device", "plug1", "address", addr)
	assertMetricValue(t, reg, "shelly_returned_energy_total_wh", 0.0, "device", "plug1", "address", addr)
	assertMetricValue(t, reg, "shelly_temperature_celsius", 43.3, "device", "plug1", "address", addr)
	assertMetricValue(t, reg, "shelly_sys_uptime_seconds", 12345, "device", "plug1", "address", addr)
}

// TestCollector_DeviceUnreachable verifies that shelly_up=0 is emitted and all other
// metrics are also present with value 0 when the device address is not reachable.
func TestCollector_DeviceUnreachable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	addr := hostPort(srv.URL)
	srv.Close()

	cfg := newTestConfig([]config.Device{{Name: "dead", Address: addr}}, 2*time.Second)
	c := New(cfg)
	c.PollAll(context.Background())
	reg := newRegistry(t, c)

	assertMetricValue(t, reg, "shelly_up", 0, "device", "dead", "address", addr)

	allExpected := []string{
		"shelly_switch_output",
		"shelly_active_power_watts",
		"shelly_voltage_volts",
		"shelly_frequency_hz",
		"shelly_current_amperes",
		"shelly_energy_total_wh",
		"shelly_returned_energy_total_wh",
		"shelly_temperature_celsius",
		"shelly_sys_uptime_seconds",
	}
	names := allMetricNames(t, reg)
	for _, name := range allExpected {
		if _, ok := names[name]; !ok {
			t.Errorf("metric %s should be present (with value 0) when device is unreachable", name)
		}
		assertMetricValue(t, reg, name, 0, "device", "dead", "address", addr)
	}
}

// TestCollector_HTTP500 verifies that shelly_up=0 and all metrics with value 0 when the device returns HTTP 500.
func TestCollector_HTTP500(t *testing.T) {
	srv := newShellyServer(t, http.StatusInternalServerError, "", http.StatusInternalServerError, "")
	defer srv.Close()

	addr := hostPort(srv.URL)
	cfg := newTestConfig([]config.Device{{Name: "errdev", Address: addr}}, 5*time.Second)
	c := New(cfg)
	c.PollAll(context.Background())
	reg := newRegistry(t, c)

	assertMetricValue(t, reg, "shelly_up", 0, "device", "errdev", "address", addr)

	names := allMetricNames(t, reg)
	allExpected := []string{
		"shelly_switch_output",
		"shelly_active_power_watts",
		"shelly_voltage_volts",
		"shelly_frequency_hz",
		"shelly_current_amperes",
		"shelly_energy_total_wh",
		"shelly_returned_energy_total_wh",
		"shelly_temperature_celsius",
		"shelly_sys_uptime_seconds",
	}
	for _, name := range allExpected {
		if _, ok := names[name]; !ok {
			t.Errorf("metric %s should be present (with value 0) on HTTP 500", name)
		}
		assertMetricValue(t, reg, name, 0, "device", "errdev", "address", addr)
	}
}

// TestCollector_PartialFailure verifies that when one of two devices fails,
// the failing device has shelly_up=0 and all metrics with value 0,
// while the healthy device has all metrics with correct values.
func TestCollector_PartialFailure(t *testing.T) {
	goodSrv := newShellyServer(t, http.StatusOK, switchStatusJSON, http.StatusOK, sysStatusJSON)
	defer goodSrv.Close()

	badSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	badAddr := hostPort(badSrv.URL)
	badSrv.Close()

	goodAddr := hostPort(goodSrv.URL)

	cfg := newTestConfig([]config.Device{
		{Name: "good", Address: goodAddr},
		{Name: "bad", Address: badAddr},
	}, 2*time.Second)
	c := New(cfg)
	c.PollAll(context.Background())
	reg := newRegistry(t, c)

	// Good device: shelly_up=1 and all metrics present.
	assertMetricValue(t, reg, "shelly_up", 1, "device", "good", "address", goodAddr)
	assertMetricValue(t, reg, "shelly_active_power_watts", 106.3, "device", "good", "address", goodAddr)
	assertMetricValue(t, reg, "shelly_voltage_volts", 230.3, "device", "good", "address", goodAddr)
	assertMetricValue(t, reg, "shelly_sys_uptime_seconds", 12345, "device", "good", "address", goodAddr)

	// Bad device: shelly_up=0 and all metrics present with value 0.
	assertMetricValue(t, reg, "shelly_up", 0, "device", "bad", "address", badAddr)

	detailedMetrics := []string{
		"shelly_switch_output",
		"shelly_active_power_watts",
		"shelly_voltage_volts",
		"shelly_frequency_hz",
		"shelly_current_amperes",
		"shelly_energy_total_wh",
		"shelly_returned_energy_total_wh",
		"shelly_temperature_celsius",
		"shelly_sys_uptime_seconds",
	}
	for _, name := range detailedMetrics {
		if !hasMetricForDevice(t, reg, name, "bad") {
			t.Errorf("metric %s should be present (with value 0) for the failed device", name)
		}
		assertMetricValue(t, reg, name, 0, "device", "bad", "address", badAddr)
	}
}

// TestCollector_Describe verifies that Describe sends all expected metric descriptors.
func TestCollector_Describe(t *testing.T) {
	cfg := newTestConfig([]config.Device{{Name: "plug1", Address: "127.0.0.1:9999"}}, 5*time.Second)
	c := New(cfg)

	ch := make(chan *prometheus.Desc, 20)
	c.Describe(ch)
	close(ch)

	count := 0
	for range ch {
		count++
	}
	if count != 10 {
		t.Errorf("expected 10 descriptors, got %d", count)
	}
}

// TestCollector_SwitchOutputOff verifies that shelly_switch_output=0 when Output is false.
func TestCollector_SwitchOutputOff(t *testing.T) {
	const switchOff = `{"output":false,"apower":0.0,"voltage":230.3,"freq":50.0,"current":0.0,"aenergy":{"total":267.813},"ret_aenergy":{"total":0.0},"temperature":{"tC":43.3}}`
	srv := newShellyServer(t, http.StatusOK, switchOff, http.StatusOK, sysStatusJSON)
	defer srv.Close()

	addr := hostPort(srv.URL)
	cfg := newTestConfig([]config.Device{{Name: "plug1", Address: addr}}, 5*time.Second)
	c := New(cfg)
	c.PollAll(context.Background())
	reg := newRegistry(t, c)

	assertMetricValue(t, reg, "shelly_switch_output", 0, "device", "plug1", "address", addr)
}

// TestCollector_CacheReturnedWhenDown verifies that after a working poll followed by a
// failing poll, all metrics are present with value 0.
func TestCollector_CacheReturnedWhenDown(t *testing.T) {
	srv := newShellyServer(t, http.StatusOK, switchStatusJSON, http.StatusOK, sysStatusJSON)
	addr := hostPort(srv.URL)

	cfg := newTestConfig([]config.Device{{Name: "plug1", Address: addr}}, 2*time.Second)
	c := New(cfg)

	c.PollAll(context.Background())
	reg := newRegistry(t, c)
	assertMetricValue(t, reg, "shelly_up", 1, "device", "plug1", "address", addr)
	assertMetricValue(t, reg, "shelly_active_power_watts", 106.3, "device", "plug1", "address", addr)

	srv.Close()

	c.PollAll(context.Background())

	assertMetricValue(t, reg, "shelly_up", 0, "device", "plug1", "address", addr)
	assertMetricValue(t, reg, "shelly_active_power_watts", 0, "device", "plug1", "address", addr)
	assertMetricValue(t, reg, "shelly_voltage_volts", 0, "device", "plug1", "address", addr)
	assertMetricValue(t, reg, "shelly_sys_uptime_seconds", 0, "device", "plug1", "address", addr)
}

// TestCollector_AllMetricsEmittedWhenDown verifies that a fresh collector with no polls
// (cache up=0) emits all 10 metrics with value 0 after polling an unreachable device.
func TestCollector_AllMetricsEmittedWhenDown(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	addr := hostPort(srv.URL)
	srv.Close()

	cfg := newTestConfig([]config.Device{{Name: "dead", Address: addr}}, 2*time.Second)
	c := New(cfg)
	c.PollAll(context.Background())
	reg := newRegistry(t, c)

	count := testutil.CollectAndCount(c)
	if count != 10 {
		t.Errorf("expected 10 metrics even when device is down, got %d", count)
	}

	allMetrics := []string{
		"shelly_up",
		"shelly_switch_output",
		"shelly_active_power_watts",
		"shelly_voltage_volts",
		"shelly_frequency_hz",
		"shelly_current_amperes",
		"shelly_energy_total_wh",
		"shelly_returned_energy_total_wh",
		"shelly_temperature_celsius",
		"shelly_sys_uptime_seconds",
	}
	names := allMetricNames(t, reg)
	for _, name := range allMetrics {
		if _, ok := names[name]; !ok {
			t.Errorf("metric %s should be present with value 0 when device is down", name)
		}
		assertMetricValue(t, reg, name, 0, "device", "dead", "address", addr)
	}
}
