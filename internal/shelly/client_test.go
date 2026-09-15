package shelly

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

const switchStatusResponse = `{"output":true,"apower":106.3,"voltage":230.3,"freq":50.0,"current":0.667,"aenergy":{"total":267.813},"ret_aenergy":{"total":0.0},"temperature":{"tC":43.3}}`
const sysStatusResponse = `{"uptime":12345}`

func newMockServer(t *testing.T, switchBody, sysBody string, switchCode, sysCode int) *httptest.Server {
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

func stripScheme(addr string) string {
	// httptest.Server.URL has http:// prefix; Client expects bare host:port
	const prefix = "http://"
	if len(addr) > len(prefix) && addr[:len(prefix)] == prefix {
		return addr[len(prefix):]
	}
	return addr
}

func TestClient_GetSwitchStatus_HappyPath(t *testing.T) {
	srv := newMockServer(t, switchStatusResponse, sysStatusResponse, http.StatusOK, http.StatusOK)
	defer srv.Close()

	client := NewClient(stripScheme(srv.URL), "", "")
	status, err := client.GetSwitchStatus(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	checks := []struct {
		name string
		got  float64
		want float64
	}{
		{"APower", status.APower, 106.3},
		{"Voltage", status.Voltage, 230.3},
		{"Freq", status.Freq, 50.0},
		{"Current", status.Current, 0.667},
		{"AEnergy.Total", status.AEnergy.Total, 267.813},
		{"RetAEnergy.Total", status.RetAEnergy.Total, 0.0},
		{"Temperature.TC", status.Temperature.TC, 43.3},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, c.got, c.want)
		}
	}
	if !status.Output {
		t.Error("expected Output=true")
	}
}

func TestClient_GetSysStatus_HappyPath(t *testing.T) {
	srv := newMockServer(t, switchStatusResponse, sysStatusResponse, http.StatusOK, http.StatusOK)
	defer srv.Close()

	client := NewClient(stripScheme(srv.URL), "", "")
	status, err := client.GetSysStatus(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status.Uptime != 12345 {
		t.Errorf("expected Uptime=12345, got %d", status.Uptime)
	}
}

func TestClient_GetSwitchStatus_HTTP500(t *testing.T) {
	srv := newMockServer(t, "", "", http.StatusInternalServerError, http.StatusOK)
	defer srv.Close()

	client := NewClient(stripScheme(srv.URL), "", "")
	_, err := client.GetSwitchStatus(context.Background())
	if err == nil {
		t.Fatal("expected error for HTTP 500, got nil")
	}
}

func TestClient_GetSysStatus_HTTP500(t *testing.T) {
	srv := newMockServer(t, switchStatusResponse, "", http.StatusOK, http.StatusInternalServerError)
	defer srv.Close()

	client := NewClient(stripScheme(srv.URL), "", "")
	_, err := client.GetSysStatus(context.Background())
	if err == nil {
		t.Fatal("expected error for HTTP 500, got nil")
	}
}

func TestClient_Unreachable(t *testing.T) {
	// Use a server that is immediately closed so the address is invalid
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	addr := stripScheme(srv.URL)
	srv.Close()

	client := NewClient(addr, "", "")
	_, err := client.GetSwitchStatus(context.Background())
	if err == nil {
		t.Fatal("expected error for unreachable device, got nil")
	}
}

func TestClient_MalformedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("not-json{{{"))
	}))
	defer srv.Close()

	client := NewClient(stripScheme(srv.URL), "", "")
	_, err := client.GetSwitchStatus(context.Background())
	if err == nil {
		t.Fatal("expected error for malformed JSON, got nil")
	}
}

func TestClient_ContextCancelled(t *testing.T) {
	srv := newMockServer(t, switchStatusResponse, sysStatusResponse, http.StatusOK, http.StatusOK)
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	client := NewClient(stripScheme(srv.URL), "", "")
	_, err := client.GetSwitchStatus(ctx)
	if err == nil {
		t.Fatal("expected error for cancelled context, got nil")
	}
}

// TestClient_RetryOnNetworkError verifies that the client retries on network errors and
// succeeds on the third attempt.
func TestClient_RetryOnNetworkError(t *testing.T) {
	var attempts atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := attempts.Add(1)
		if n < 3 {
			// Close connection abruptly to simulate a network error.
			hj, ok := w.(http.Hijacker)
			if !ok {
				t.Error("responsewriter does not support hijacking")
				return
			}
			conn, _, _ := hj.Hijack()
			conn.Close()
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(switchStatusResponse))
	}))
	defer srv.Close()

	client := NewClient(stripScheme(srv.URL), "", "")
	_, err := client.GetSwitchStatus(context.Background())
	if err != nil {
		t.Fatalf("expected success on third attempt, got error: %v", err)
	}
	if got := attempts.Load(); got != 3 {
		t.Errorf("expected 3 attempts, got %d", got)
	}
}

// TestClient_NoRetryOnHTTP500 verifies that the client does NOT retry on HTTP 5xx responses.
func TestClient_NoRetryOnHTTP500(t *testing.T) {
	var attempts atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := NewClient(stripScheme(srv.URL), "", "")
	_, err := client.GetSwitchStatus(context.Background())
	if err == nil {
		t.Fatal("expected error for HTTP 500, got nil")
	}
	if got := attempts.Load(); got != 1 {
		t.Errorf("expected exactly 1 attempt (no retry on HTTP 500), got %d", got)
	}
}
