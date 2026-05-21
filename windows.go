//go:build windows

package main

import "amdtop/adlxwrap"

func initGPU() error {
	return adlxwrap.Init()
}

func collectGPUData() (*GPUData, error) {
	d, err := adlxwrap.Collect()
	if err != nil {
		return nil, err
	}
	return &GPUData{
		Name:        d.Name,
		Utilization: d.Utilization,
		Temperature: d.Temperature,
		Power:       d.Power,
		PowerCap:    d.PowerCap,
		VRAMUsed:    d.VRAMUsed,
		VRAMTotal:   d.VRAMTotal,
	}, nil
}

func closeGPU() {
	adlxwrap.Close()
}
