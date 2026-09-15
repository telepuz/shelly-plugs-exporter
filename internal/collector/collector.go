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

type deviceResult struct {
	device       config.Device
	up           float64
	switchStatus *shelly.SwitchStatus
	sysStatus    *shelly.SysStatus
}

type Collector struct {
	devices       []config.Device
	scrapeTimeout time.Duration
}

func New(cfg *config.Config) *Collector {
	return &Collector{
		devices:       cfg.Devices,
		scrapeTimeout: cfg.ScrapeTimeout,
	}
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
	ctx, cancel := context.WithTimeout(context.Background(), c.scrapeTimeout)
	defer cancel()

	results := make([]deviceResult, len(c.devices))
	var wg sync.WaitGroup

	for i, dev := range c.devices {
		wg.Add(1)
		go func(idx int, d config.Device) {
			defer wg.Done()
			results[idx] = pollDevice(ctx, d)
		}(i, dev)
	}
	wg.Wait()

	for _, r := range results {
		name := r.device.Name
		addr := r.device.Address

		ch <- prometheus.MustNewConstMetric(descUp, prometheus.GaugeValue, r.up, name, addr)

		if r.up == 0 {
			continue
		}

		if r.switchStatus != nil {
			switchOutput := 0.0
			if r.switchStatus.Output {
				switchOutput = 1.0
			}
			ch <- prometheus.MustNewConstMetric(descSwitchOutput, prometheus.GaugeValue, switchOutput, name, addr)
			ch <- prometheus.MustNewConstMetric(descActivePower, prometheus.GaugeValue, r.switchStatus.APower, name, addr)
			ch <- prometheus.MustNewConstMetric(descVoltage, prometheus.GaugeValue, r.switchStatus.Voltage, name, addr)
			ch <- prometheus.MustNewConstMetric(descFrequency, prometheus.GaugeValue, r.switchStatus.Freq, name, addr)
			ch <- prometheus.MustNewConstMetric(descCurrent, prometheus.GaugeValue, r.switchStatus.Current, name, addr)
			ch <- prometheus.MustNewConstMetric(descEnergyTotal, prometheus.GaugeValue, r.switchStatus.AEnergy.Total, name, addr)
			ch <- prometheus.MustNewConstMetric(descReturnedEnergyTotal, prometheus.GaugeValue, r.switchStatus.RetAEnergy.Total, name, addr)
			ch <- prometheus.MustNewConstMetric(descTemperature, prometheus.GaugeValue, r.switchStatus.Temperature.TC, name, addr)
		}

		if r.sysStatus != nil {
			ch <- prometheus.MustNewConstMetric(descUptime, prometheus.GaugeValue, float64(r.sysStatus.Uptime), name, addr)
		}
	}
}

func pollDevice(ctx context.Context, dev config.Device) deviceResult {
	client := shelly.NewClient(dev.Address, dev.Username, dev.Password)

	switchStatus, err := client.GetSwitchStatus(ctx)
	if err != nil {
		slog.Warn("failed to get switch status", "device", dev.Name, "address", dev.Address, "err", err)
		return deviceResult{device: dev, up: 0}
	}

	sysStatus, err := client.GetSysStatus(ctx)
	if err != nil {
		slog.Warn("failed to get sys status", "device", dev.Name, "address", dev.Address, "err", err)
		return deviceResult{device: dev, up: 0}
	}

	return deviceResult{
		device:       dev,
		up:           1,
		switchStatus: switchStatus,
		sysStatus:    sysStatus,
	}
}
