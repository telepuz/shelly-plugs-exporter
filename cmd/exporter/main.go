package main

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/telepuz/shelly-plugs-exporter/internal/collector"
	"github.com/telepuz/shelly-plugs-exporter/internal/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "err", err)
		panic(err)
	}

	slog.Info("starting shelly-plugs-exporter",
		"listen", cfg.ListenAddress,
		"devices", len(cfg.Devices),
		"scrape_timeout", cfg.ScrapeTimeout,
		"poll_interval", cfg.PollInterval,
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	c := collector.New(cfg)
	c.Start(ctx)

	reg := prometheus.NewRegistry()
	reg.MustRegister(c)

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	if err := http.ListenAndServe(cfg.ListenAddress, mux); err != nil {
		slog.Error("HTTP server error", "err", err)
		panic(err)
	}
}
