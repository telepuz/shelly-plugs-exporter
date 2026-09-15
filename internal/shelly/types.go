package shelly

type EnergyCounter struct {
	Total float64 `json:"total"`
}

type Temperature struct {
	TC float64 `json:"tC"`
}

type SwitchStatus struct {
	Output     bool          `json:"output"`
	APower     float64       `json:"apower"`
	Voltage    float64       `json:"voltage"`
	Freq       float64       `json:"freq"`
	Current    float64       `json:"current"`
	AEnergy    EnergyCounter `json:"aenergy"`
	RetAEnergy EnergyCounter `json:"ret_aenergy"`
	Temperature Temperature  `json:"temperature"`
}

type SysStatus struct {
	Uptime int64 `json:"uptime"`
}
