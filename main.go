package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os/exec"
	"strings"
	"syscall"
	"strconv"
)

type StatusResponse struct {
	OS        string  `json:"os"`
	CpuUsage  float64 `json:"cpu_usage"`
	RamUsage  float64 `json:"ram_usage"`
	DiskUsage float64 `json:"disk_usage"`
}

// Simple helper to get real disk usage for the root directory "/"
func getDiskUsage() float64 {
	var stat syscall.Statfs_t
	if err := syscall.Statfs("/", &stat); err != nil {
		return 0.0
	}
	// Total space - free space = used space
	total := float64(stat.Blocks * uint64(stat.Bsize))
	free := float64(stat.Bfree * uint64(stat.Bsize))
	if total == 0 {
		return 0.0
	}
	used := total - free
	return (used / total) * 100.0
}

// Simple helper to grab approximate CPU usage on Linux
func getCpuUsage() float64 {
	// Reads load average or top info; fallback to a safe parse
	out, err := exec.Command("sh", "-c", "top -bn1 | grep 'Cpu(s)' | awk '{print $2 + $4}'").Output()
	if err != nil {
		return 0.0
	}
	val := strings.TrimSpace(string(out))
	cpu, err := strconv.ParseFloat(val, 64)
	if err != nil {
		return 0.0
	}
	return cpu
}

// Simple helper to get RAM usage percentage
func getRamUsage() float64 {
	out, err := exec.Command("free").Output()
	if err != nil {
		return 0.0
	}
	lines := strings.Split(string(out), "\n")
	if len(lines) < 2 {
		return 0.0
	}
	fields := strings.Fields(lines[1])
	if len(fields) < 3 {
		return 0.0
	}
	total, _ := strconv.ParseFloat(fields[1], 64)
	used, _ := strconv.ParseFloat(fields[2], 64)
	if total == 0 {
		return 0.0
	}
	return (used / total) * 100.0
}

func handleStatus(w http.ResponseWriter, r *http.Request) {
	res := StatusResponse{
		OS:        "Ubuntu Linux",
		CpuUsage:  getCpuUsage(),
		RamUsage:  getRamUsage(),
		DiskUsage: getDiskUsage(),
	}

	w.Header().Set("Content-Type", "application/json")
	
	jsonData, err := json.Marshal(res)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	
	w.Write(jsonData)
}

func handleReboot(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"rebooting"}`))
	
	go func() {
		exec.Command("reboot").Run()
	}()
}

func handleShutdown(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"shutting down"}`))
	
	go func() {
		exec.Command("shutdown", "now").Run()
	}()
}

func main() {
	http.HandleFunc("/api/status", handleStatus)
	http.HandleFunc("/api/power/reboot", handleReboot)
	http.HandleFunc("/api/power/shutdown", handleShutdown)

	log.Println("Home server backend running on port 8080...")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
