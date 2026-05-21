//go:build linux

package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

var (
	gpuDevPath string
	hwmonPath  string
	gpuName    string
)

// amdGPUModels maps PCI device IDs to marketing names.
var amdGPUModels = map[string]string{
	"15bf": "Radeon 780M",
	"1681": "Radeon 680M",
	"1682": "Radeon 660M",
	"164e": "Radeon 710M",
	"99af": "Radeon 610M",
	"1638": "Radeon Graphics (Cezanne)",
	"744c": "Radeon RX 7900 XTX",
	"7448": "Radeon RX 7900 XT",
	"7444": "Radeon RX 7900 GRE",
	"747e": "Radeon RX 7900 GRE",
	"7480": "Radeon RX 7600",
	"7483": "Radeon RX 7600 XT",
	"73bf": "Radeon RX 6800",
	"73c3": "Radeon RX 6800 XT",
	"73df": "Radeon RX 6900 XT",
	"73ff": "Radeon RX 6600",
	"73fe": "Radeon RX 6600 XT",
	"73ef": "Radeon RX 6700 XT",
	"73e3": "Radeon RX 6700",
	"731f": "Radeon RX 5700 XT",
	"7310": "Radeon RX 5700",
	"7312": "Radeon RX 5600 XT",
	"67df": "Radeon RX 580",
	"67ef": "Radeon RX 590",
	"67c7": "Radeon RX 5500 XT",
}

func initGPU() error {
	matches, err := filepath.Glob("/sys/class/drm/card*/device")
	if err != nil {
		return fmt.Errorf("can't access drm devices: %v", err)
	}
	for _, dev := range matches {
		vendor, err := os.ReadFile(filepath.Join(dev, "vendor"))
		if err != nil {
			continue
		}
		if strings.TrimSpace(string(vendor)) == "0x1002" {
			gpuDevPath = dev
			break
		}
	}
	if gpuDevPath == "" {
		return errors.New("no AMD GPU found (vendor ≠ 0x1002)")
	}

	var pciSlot string
	if uev, err := os.ReadFile(filepath.Join(gpuDevPath, "uevent")); err == nil {
		for line := range strings.SplitSeq(string(uev), "\n") {
			if slot, ok := strings.CutPrefix(line, "PCI_SLOT_NAME="); ok {
				pciSlot = slot
			}
			if id, ok := strings.CutPrefix(line, "PCI_ID="); ok {
				if _, devID, found := strings.Cut(id, ":"); found {
					devID = strings.ToLower(devID)
					if name, exists := amdGPUModels[devID]; exists {
						gpuName = name
					}
				}
			}
		}
	}

	if gpuName == "" && pciSlot != "" {
		out, err := exec.Command("lspci", "-s", pciSlot, "-nn").Output()
		if err == nil && len(out) > 0 {
			if name := parseLspciName(string(out)); name != "" {
				gpuName = name
			}
		}
	}
	if gpuName == "" {
		if devBytes, _ := os.ReadFile(filepath.Join(gpuDevPath, "device")); len(devBytes) > 0 {
			gpuName = fmt.Sprintf("AMD GPU (0x%s)", strings.TrimSpace(string(devBytes)))
		} else {
			gpuName = "AMD GPU"
		}
	}

	hwmonBase := filepath.Join(gpuDevPath, "hwmon")
	dirs, err := os.ReadDir(hwmonBase)
	if err == nil {
		for _, d := range dirs {
			nameBytes, err := os.ReadFile(filepath.Join(hwmonBase, d.Name(), "name"))
			if err != nil {
				continue
			}
			if strings.TrimSpace(string(nameBytes)) == "amdgpu" {
				hwmonPath = filepath.Join(hwmonBase, d.Name())
				break
			}
		}
	}
	return nil
}

// parseLspciName extracts the device name from lspci output.
//
//	input: "63:00.0 VGA compatible controller [0300]: Advanced Micro Devices, Inc. [AMD/ATI] Phoenix1 [1002:15bf] (rev c7)"
//	output: "Phoenix1"
func parseLspciName(line string) string {
	_, after, ok := strings.Cut(line, "]: ")
	if !ok {
		return ""
	}

	var devName string
	for _, prefix := range []string{"[AMD/ATI] ", "[AMD] "} {
		if _, after, ok = strings.Cut(after, prefix); ok {
			devName = after
			break
		}
	}
	if !ok {
		devName = after
	}

	for _, sep := range []string{" [", " (", "\n"} {
		if before, _, ok := strings.Cut(devName, sep); ok {
			devName = before
		}
	}
	return strings.TrimSpace(devName)
}

func collectGPUData() (*GPUData, error) {
	d := &GPUData{Name: gpuName}

	if b, err := os.ReadFile(filepath.Join(gpuDevPath, "gpu_busy_percent")); err == nil {
		v, _ := strconv.Atoi(strings.TrimSpace(string(b)))
		d.Utilization = v
	}

	if hwmonPath != "" {
		if b, err := os.ReadFile(filepath.Join(hwmonPath, "temp1_input")); err == nil {
			v, _ := strconv.ParseFloat(strings.TrimSpace(string(b)), 64)
			d.Temperature = v / 1000.0
		}
		if b, err := os.ReadFile(filepath.Join(hwmonPath, "power1_average")); err == nil {
			v, _ := strconv.ParseFloat(strings.TrimSpace(string(b)), 64)
			d.Power = v / 1_000_000.0
		}
		if b, err := os.ReadFile(filepath.Join(hwmonPath, "power1_cap")); err == nil {
			v, _ := strconv.ParseFloat(strings.TrimSpace(string(b)), 64)
			d.PowerCap = v / 1_000_000.0
		}
	}

	if b, err := os.ReadFile(filepath.Join(gpuDevPath, "mem_info_vram_total")); err == nil {
		d.VRAMTotal, _ = strconv.ParseUint(strings.TrimSpace(string(b)), 10, 64)
	}
	if b, err := os.ReadFile(filepath.Join(gpuDevPath, "mem_info_vram_used")); err == nil {
		d.VRAMUsed, _ = strconv.ParseUint(strings.TrimSpace(string(b)), 10, 64)
	}

	return d, nil
}

func closeGPU() {}
