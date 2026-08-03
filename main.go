package main

import (
	"encoding/json"
	"net/http"
	"os/exec"
	"runtime"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/mem"
)

type SystemStatus struct {
	OS        string  `json:"os"`
	CPUUsage  float64 `json:"cpu_usage"`
	RAMUsage  float64 `json:"ram_usage"`
	DiskUsage float64 `json:"disk_usage"`
}

func authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		apiKey := r.Header.Get("X-API-Key")
		if apiKey != "YOUR_SUPER_SECRET_KEY" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

func handleStatus(w http.ResponseWriter, r *http.Request) {
	cpuPercents, _ := cpu.Percent(time.Second, false)
	cpuVal := 0.0
	if len(cpuPercents) > 0 {
		cpuVal = cpuPercents[0]
	}

	vmStat, _ := mem.VirtualMemory()
	ramVal := vmStat.UsedPercent

	diskStat, _ := disk.Usage("/")
	diskVal := 0.0
	if diskStat != nil {
		diskVal = diskStat.UsedPercent
	}

	res := SystemStatus{
		OS:        "Ubuntu " + runtime.GOOS,
		CPUUsage:  cpuVal,
		RAMUsage:  ramVal,
		DiskUsage: diskVal,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w.Encode(res))
}

func handleShutdown(w http.ResponseWriter, r *http.Request) {
	cmd := exec.Command("systemctl", "poweroff")
	err := cmd.Run()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Write([]byte("Ubuntu server shutting down..."))
}

func handleReboot(w http.ResponseWriter, r *http.Request) {
	cmd := exec.Command("systemctl", "reboot")
	err := cmd.Run()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Write([]byte("Ubuntu server rebooting..."))
}

func main() {
	http.HandleFunc("/api/status", authMiddleware(handleStatus))
	http.HandleFunc("/api/power/shutdown", authMiddleware(handleShutdown))
	http.HandleFunc("/api/power/reboot", authMiddleware(handleReboot))

	// Replace with your Tailscale IP or use "0.0.0.0:8080" to listen on all interfaces
	http.ListenAndServe("100.x.x.x:8080", nil)
}
