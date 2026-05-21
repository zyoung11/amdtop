package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"
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
	addr := "0.0.0.0:" + strconv.Itoa(port)
	go http.ListenAndServe(addr, nil)
}

func checkConnect(ip string, port int) error {
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get("http://" + ip + ":" + strconv.Itoa(port) + "/api/v1/metrics")
	if err != nil {
		return fmt.Errorf("cannot connect to %s:%d: %v", ip, port, err)
	}
	resp.Body.Close()
	return nil
}

func checkPortReserved(port int) error {
	if runtime.GOOS != "windows" {
		return nil
	}
	out, err := exec.Command("powershell", "-NoProfile", "-Command",
		"netsh int ipv4 show excludedportrange tcp").Output()
	if err != nil {
		return nil
	}
	for line := range strings.SplitSeq(string(out), "\n") {
		f := strings.Fields(line)
		if len(f) >= 2 {
			start, e1 := strconv.Atoi(f[0])
			end, e2 := strconv.Atoi(f[1])
			if e1 == nil && e2 == nil && port >= start && port <= end {
				return fmt.Errorf("port %d falls in Windows reserved range %d-%d (run 'netsh int ipv4 show excludedportrange tcp' as admin to view all)", port, start, end)
			}
		}
	}
	return nil
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
