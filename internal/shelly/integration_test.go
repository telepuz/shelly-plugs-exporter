//go:build integration

package shelly

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/joho/godotenv"
)

// Run with: go test -tags integration ./internal/shelly/
// Required env vars:
//   SHELLY_TEST_ADDRESS  (default: 192.168.0.106)
//   SHELLY_TEST_USERNAME (default: admin)
//   SHELLY_TEST_PASSWORD (required, no default)

func TestMain(m *testing.M) {
	// load .env from repo root if it exists; ignore error if absent
	_ = godotenv.Load("../../.env")
	os.Exit(m.Run())
}

func testClient(t *testing.T) *Client {
	t.Helper()
	address := os.Getenv("SHELLY_TEST_ADDRESS")
	if address == "" {
		address = "192.168.0.106"
	}
	username := os.Getenv("SHELLY_TEST_USERNAME")
	if username == "" {
		username = "admin"
	}
	password := os.Getenv("SHELLY_TEST_PASSWORD")
	if password == "" {
		t.Fatal("SHELLY_TEST_PASSWORD env var required for integration tests")
	}
	return NewClient(address, username, password)
}

func TestIntegration_GetSwitchStatus(t *testing.T) {
	client := testClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	status, err := client.GetSwitchStatus(ctx)
	if err != nil {
		t.Fatalf("GetSwitchStatus: %v", err)
	}
	t.Logf("output=%v apower=%.1f voltage=%.1f freq=%.1f current=%.3f energy_total=%.3f temp=%.1f",
		status.Output, status.APower, status.Voltage, status.Freq,
		status.Current, status.AEnergy.Total, status.Temperature.TC)
}

func TestIntegration_GetSysStatus(t *testing.T) {
	client := testClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	status, err := client.GetSysStatus(ctx)
	if err != nil {
		t.Fatalf("GetSysStatus: %v", err)
	}
	t.Logf("uptime=%d", status.Uptime)
}
