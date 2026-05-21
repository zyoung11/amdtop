package main

import (
	"encoding/json"
	"net/http"
	"strconv"
)

type metricsJSON struct {
	Name        string  `json:"name"`
	Utilization int     `json:"utilization"`
	Temperature float64 `json:"temperature"`
	Power       float64 `json:"power"`
	PowerCap    float64 `json:"power_cap"`
	VRAMUsed    uint64  `json:"vram_used"`
	VRAMTotal   uint64  `json:"vram_total"`
}

var currentMetrics metricsJSON

func startServer(port int) {
	http.HandleFunc("/api/v1/metrics", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(currentMetrics)
	})
	go http.ListenAndServe(":"+strconv.Itoa(port), nil)
}

func fetchMetrics(ip string, port int) (*GPUData, error) {
	resp, err := http.Get("http://" + ip + ":" + strconv.Itoa(port) + "/api/v1/metrics")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var m metricsJSON
	if err := json.NewDecoder(resp.Body).Decode(&m); err != nil {
		return nil, err
	}
	return &GPUData{
		Name:        m.Name,
		Utilization: m.Utilization,
		Temperature: m.Temperature,
		Power:       m.Power,
		PowerCap:    m.PowerCap,
		VRAMUsed:    m.VRAMUsed,
		VRAMTotal:   m.VRAMTotal,
	}, nil
}
