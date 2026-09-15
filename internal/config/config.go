package config

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

type Device struct {
	Name     string `json:"name"`
	Address  string `json:"address"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
}

type Config struct {
	Devices       []Device
	ListenAddress string
	ScrapeTimeout time.Duration
	PollInterval  time.Duration
}

func Load() (*Config, error) {
	devicesJSON := os.Getenv("DEVICES")
	if devicesJSON == "" {
		return nil, fmt.Errorf("DEVICES env var is required")
	}

	var devices []Device
	if err := json.Unmarshal([]byte(devicesJSON), &devices); err != nil {
		return nil, fmt.Errorf("parsing DEVICES: %w", err)
	}
	if len(devices) == 0 {
		return nil, fmt.Errorf("DEVICES must contain at least one device")
	}

	listenAddress := os.Getenv("LISTEN_ADDRESS")
	if listenAddress == "" {
		listenAddress = ":9924"
	}

	scrapeTimeoutStr := os.Getenv("SCRAPE_TIMEOUT")
	if scrapeTimeoutStr == "" {
		scrapeTimeoutStr = "30s"
	}
	scrapeTimeout, err := time.ParseDuration(scrapeTimeoutStr)
	if err != nil {
		return nil, fmt.Errorf("parsing SCRAPE_TIMEOUT: %w", err)
	}

	pollIntervalStr := os.Getenv("POLL_INTERVAL")
	if pollIntervalStr == "" {
		pollIntervalStr = "15s"
	}
	pollInterval, err := time.ParseDuration(pollIntervalStr)
	if err != nil {
		return nil, fmt.Errorf("parsing POLL_INTERVAL: %w", err)
	}

	return &Config{
		Devices:       devices,
		ListenAddress: listenAddress,
		ScrapeTimeout: scrapeTimeout,
		PollInterval:  pollInterval,
	}, nil
}
