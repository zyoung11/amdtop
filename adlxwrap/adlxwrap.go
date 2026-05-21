//go:build windows

package adlxwrap

/*
#include "adlx_bridge.h"
*/
import "C"
import "errors"

// Data holds one sample of all GPU metrics.
type Data struct {
	Name        string
	Utilization int
	Temperature float64
	Power       float64
	PowerCap    float64
	VRAMUsed    uint64
	VRAMTotal   uint64
}

// Init initialises ADLX.
func Init() error {
	if C.adlx_init() != 0 {
		return errors.New("ADLX init failed")
	}
	return nil
}

// Collect fetches one sample of GPU metrics.
func Collect() (*Data, error) {
	var raw C.ADLXGPUData
	if C.adlx_collect(&raw) != 0 {
		return nil, errors.New("ADLX collect failed")
	}
	return &Data{
		Name:        C.GoString(&raw.name[0]),
		Utilization: int(raw.gpuUsage + 0.5),
		Temperature: float64(raw.gpuTemp),
		Power:       float64(raw.gpuPower),
		PowerCap:    float64(raw.gpuPowerCap),
		VRAMUsed:    uint64(raw.vramUsedMB) * 1024 * 1024,
		VRAMTotal:   uint64(raw.vramTotalMB) * 1024 * 1024,
	}, nil
}

// Close shuts down ADLX.
func Close() {
	C.adlx_close()
}
