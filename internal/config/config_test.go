package config

import (
	"testing"
	"time"
)

func TestLoad_ValidDevices(t *testing.T) {
	t.Setenv("DEVICES", `[{"name":"plug1","address":"192.168.1.10"}]`)
	t.Setenv("LISTEN_ADDRESS", "")
	t.Setenv("SCRAPE_TIMEOUT", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(cfg.Devices) != 1 {
		t.Fatalf("expected 1 device, got %d", len(cfg.Devices))
	}
	if cfg.Devices[0].Name != "plug1" {
		t.Errorf("expected device name plug1, got %s", cfg.Devices[0].Name)
	}
	if cfg.Devices[0].Address != "192.168.1.10" {
		t.Errorf("expected address 192.168.1.10, got %s", cfg.Devices[0].Address)
	}
	if cfg.ListenAddress != ":9924" {
		t.Errorf("expected default listen address :9924, got %s", cfg.ListenAddress)
	}
	if cfg.ScrapeTimeout != 30*time.Second {
		t.Errorf("expected default scrape timeout 30s, got %v", cfg.ScrapeTimeout)
	}
}

func TestLoad_MultipleDevicesWithAuth(t *testing.T) {
	devJSON := `[
		{"name":"plug1","address":"10.0.0.1","username":"admin","password":"secret"},
		{"name":"plug2","address":"10.0.0.2"}
	]`
	t.Setenv("DEVICES", devJSON)
	t.Setenv("LISTEN_ADDRESS", ":8080")
	t.Setenv("SCRAPE_TIMEOUT", "5s")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(cfg.Devices) != 2 {
		t.Fatalf("expected 2 devices, got %d", len(cfg.Devices))
	}
	if cfg.Devices[0].Username != "admin" {
		t.Errorf("expected username admin, got %s", cfg.Devices[0].Username)
	}
	if cfg.Devices[0].Password != "secret" {
		t.Errorf("expected password secret, got %s", cfg.Devices[0].Password)
	}
	if cfg.ListenAddress != ":8080" {
		t.Errorf("expected :8080, got %s", cfg.ListenAddress)
	}
	if cfg.ScrapeTimeout != 5*time.Second {
		t.Errorf("expected 5s, got %v", cfg.ScrapeTimeout)
	}
}

func TestLoad_MissingDevicesEnv(t *testing.T) {
	t.Setenv("DEVICES", "")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error when DEVICES is missing, got nil")
	}
}

func TestLoad_MalformedJSON(t *testing.T) {
	t.Setenv("DEVICES", `not-valid-json`)

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for malformed JSON, got nil")
	}
}

func TestLoad_EmptyDevicesList(t *testing.T) {
	t.Setenv("DEVICES", `[]`)

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for empty devices list, got nil")
	}
}

func TestLoad_InvalidScrapeTimeout(t *testing.T) {
	t.Setenv("DEVICES", `[{"name":"plug1","address":"192.168.1.1"}]`)
	t.Setenv("SCRAPE_TIMEOUT", "not-a-duration")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for invalid SCRAPE_TIMEOUT, got nil")
	}
}

func TestLoad_CustomListenAddress(t *testing.T) {
	t.Setenv("DEVICES", `[{"name":"plug1","address":"192.168.1.1"}]`)
	t.Setenv("LISTEN_ADDRESS", ":2112")
	t.Setenv("SCRAPE_TIMEOUT", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cfg.ListenAddress != ":2112" {
		t.Errorf("expected :2112, got %s", cfg.ListenAddress)
	}
}
