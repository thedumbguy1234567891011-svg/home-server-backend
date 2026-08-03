package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os/exec"
)

type StatusResponse struct {
	OS        string  `json:"os"`
	CpuUsage  float64 `json:"cpu_usage"`
	RamUsage  float64 `json:"ram_usage"`
	DiskUsage float64 `json:"disk_usage"`
}

func handleStatus(w http.ResponseWriter, r *http.Request) {
	res := StatusResponse{
		OS:        "Ubuntu Linux",
		CpuUsage:  42.5,
		RamUsage:  60.2,
		DiskUsage: 55.0,
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
