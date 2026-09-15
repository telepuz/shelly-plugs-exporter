package collector

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/telepuz/shelly-plugs-exporter/internal/config"
	"github.com/telepuz/shelly-plugs-exporter/internal/shelly"
)

var labelNames = []string{"device", "address"}

var (
	descUp = prometheus.NewDesc(
		"shelly_up",
		"1 if device is reachable, 0 otherwise.",
		labelNames, nil,
	)
	descSwitchOutput = prometheus.NewDesc(
		"shelly_switch_output",
		"Switch output state (1=on, 0=off).",
		labelNames, nil,
	)
	descActivePower = prometheus.NewDesc(
		"shelly_active_power_watts",
		"Active power consumption in watts.",
		labelNames, nil,
	)
	descVoltage = prometheus.NewDesc(
		"shelly_voltage_volts",
		"Measured voltage in volts.",
		labelNames, nil,
	)
	descFrequency = prometheus.NewDesc(
		"shelly_frequency_hz",
		"AC frequency in hertz.",
		labelNames, nil,
	)
	descCurrent = prometheus.NewDesc(
		"shelly_current_amperes",
		"Measured current in amperes.",
		labelNames, nil,
	)
	descEnergyTotal = prometheus.NewDesc(
		"shelly_energy_total_wh",
		"Total energy consumed in watt-hours.",
		labelNames, nil,
	)
	descReturnedEnergyTotal = prometheus.NewDesc(
		"shelly_returned_energy_total_wh",
		"Total energy returned in watt-hours.",
		labelNames, nil,
	)
	descTemperature = prometheus.NewDesc(
		"shelly_temperature_celsius",
		"Device temperature in Celsius.",
		labelNames, nil,
	)
	descUptime = prometheus.NewDesc(
		"shelly_sys_uptime_seconds",
		"System uptime in seconds.",
		labelNames, nil,
	)
)

type deviceCache struct {
	mu           sync.RWMutex
	up           float64
	switchStatus shelly.SwitchStatus
	sysStatus    shelly.SysStatus
}

type Collector struct {
	devices       []config.Device
	scrapeTimeout time.Duration
	pollInterval  time.Duration
	caches        []*deviceCache
}

func New(cfg *config.Config) *Collector {
	caches := make([]*deviceCache, len(cfg.Devices))
	for i := range caches {
		caches[i] = &deviceCache{}
	}
	return &Collector{
		devices:       cfg.Devices,
		scrapeTimeout: cfg.ScrapeTimeout,
		pollInterval:  cfg.PollInterval,
		caches:        caches,
	}
}

func (c *Collector) Start(ctx context.Context) {
	for i := range c.devices {
		go func(idx int) {
			c.pollAndCache(ctx, idx)
			ticker := time.NewTicker(c.pollInterval)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					c.pollAndCache(ctx, idx)
				}
			}
		}(i)
	}
}

func (c *Collector) PollAll(ctx context.Context) {
	var wg sync.WaitGroup
	for i := range c.devices {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			c.pollAndCache(ctx, idx)
		}(i)
	}
	wg.Wait()
}

func (c *Collector) pollAndCache(ctx context.Context, idx int) {
	dev := c.devices[idx]
	cache := c.caches[idx]

	pollCtx, cancel := context.WithTimeout(ctx, c.scrapeTimeout)
	defer cancel()

	client := shelly.NewClient(dev.Address, dev.Username, dev.Password)

	switchStatus, err := client.GetSwitchStatus(pollCtx)
	if err != nil {
		slog.Warn("failed to get switch status", "device", dev.Name, "address", dev.Address, "err", err)
		cache.mu.Lock()
		cache.up = 0
		cache.switchStatus = shelly.SwitchStatus{}
		cache.sysStatus = shelly.SysStatus{}
		cache.mu.Unlock()
		return
	}

	sysStatus, err := client.GetSysStatus(pollCtx)
	if err != nil {
		slog.Warn("failed to get sys status", "device", dev.Name, "address", dev.Address, "err", err)
		cache.mu.Lock()
		cache.up = 0
		cache.switchStatus = shelly.SwitchStatus{}
		cache.sysStatus = shelly.SysStatus{}
		cache.mu.Unlock()
		return
	}

	cache.mu.Lock()
	cache.up = 1
	cache.switchStatus = *switchStatus
	cache.sysStatus = *sysStatus
	cache.mu.Unlock()
}

func (c *Collector) Describe(ch chan<- *prometheus.Desc) {
	ch <- descUp
	ch <- descSwitchOutput
	ch <- descActivePower
	ch <- descVoltage
	ch <- descFrequency
	ch <- descCurrent
	ch <- descEnergyTotal
	ch <- descReturnedEnergyTotal
	ch <- descTemperature
	ch <- descUptime
}

func (c *Collector) Collect(ch chan<- prometheus.Metric) {
	for i, dev := range c.devices {
		name := dev.Name
		addr := dev.Address
		cache := c.caches[i]

		cache.mu.RLock()
		up := cache.up
		ss := cache.switchStatus
		sys := cache.sysStatus
		cache.mu.RUnlock()

		ch <- prometheus.MustNewConstMetric(descUp, prometheus.GaugeValue, up, name, addr)

		switchOutput := 0.0
		if ss.Output {
			switchOutput = 1.0
		}
		ch <- prometheus.MustNewConstMetric(descSwitchOutput, prometheus.GaugeValue, switchOutput, name, addr)
		ch <- prometheus.MustNewConstMetric(descActivePower, prometheus.GaugeValue, ss.APower, name, addr)
		ch <- prometheus.MustNewConstMetric(descVoltage, prometheus.GaugeValue, ss.Voltage, name, addr)
		ch <- prometheus.MustNewConstMetric(descFrequency, prometheus.GaugeValue, ss.Freq, name, addr)
		ch <- prometheus.MustNewConstMetric(descCurrent, prometheus.GaugeValue, ss.Current, name, addr)
		ch <- prometheus.MustNewConstMetric(descEnergyTotal, prometheus.GaugeValue, ss.AEnergy.Total, name, addr)
		ch <- prometheus.MustNewConstMetric(descReturnedEnergyTotal, prometheus.GaugeValue, ss.RetAEnergy.Total, name, addr)
		ch <- prometheus.MustNewConstMetric(descTemperature, prometheus.GaugeValue, ss.Temperature.TC, name, addr)
		ch <- prometheus.MustNewConstMetric(descUptime, prometheus.GaugeValue, float64(sys.Uptime), name, addr)
	}
}
