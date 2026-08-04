package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type ServerStatus struct {
	OS        string  `json:"os"`
	CPUUsage  float64 `json:"cpu_usage"`
	RAMUsage  float64 `json:"ram_usage"`
	DiskUsage float64 `json:"disk_usage"`
}

type FileInfo struct {
	Name      string `json:"name"`
	IsDir     bool   `json:"is_dir"`
	Extension string `json:"extension"`
	Size      string `json:"size"`
}

type DirResponse struct {
	Path       string     `json:"path"`
	ParentPath string     `json:"parent_path"`
	Files      []FileInfo `json:"files"`
}

func main() {
	http.HandleFunc("/api/status/live", handleLiveStatus)
	http.HandleFunc("/api/files", handleFiles)

	fmt.Println("Home server daemon running on port 8080...")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		fmt.Printf("Server failed: %v\n", err)
	}
}

func getOSName() string {
	data, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return "Ubuntu Server"
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "PRETTY_NAME=") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				return strings.Trim(parts[1], `"`)
			}
		}
	}
	return "Ubuntu Server"
}

func handleLiveStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	osName := getOSName()

	for {
		status := ServerStatus{
			OS:        osName,
			CPUUsage:  getCPUUsage(),
			RAMUsage:  getRAMUsage(),
			DiskUsage: getDiskUsage(),
		}

		jsonData, err := json.Marshal(status)
		if err == nil {
			fmt.Fprintf(w, "data: %s\n\n", string(jsonData))
			flusher.Flush()
		}

		time.Sleep(2 * time.Second)
	}
}

func handleFiles(w http.ResponseWriter, r *http.Request) {
	reqPath := r.URL.Query().Get("path")
	if reqPath == "" {
		reqPath = "/"
	}

	cleanPath := filepath.Clean(reqPath)
	entries, err := os.ReadDir(cleanPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	parentPath := filepath.Dir(cleanPath)
	if parentPath == cleanPath {
		parentPath = ""
	}

	var files []FileInfo
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}

		name := entry.Name()
		isDir := entry.IsDir()
		ext := ""
		sizeStr := ""

		if !isDir {
			ext = strings.TrimPrefix(filepath.Ext(name), ".")
			sizeStr = formatBytes(info.Size())
		} else {
			sizeStr = "--"
		}

		files = append(files, FileInfo{
			Name:      name,
			IsDir:     isDir,
			Extension: ext,
			Size:      sizeStr,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(DirResponse{
		Path:       cleanPath,
		ParentPath: parentPath,
		Files:      files,
	})
}

func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// Real metric fetchers using Linux commands
func getCPUUsage() float64 {
	// Parses CPU idle time using mpstat or top batch mode, fallback method using top
	cmd := exec.Command("sh", "-c", "top -bn1 | grep 'Cpu(s)' | awk '{print 100 - $8}'")
	out, err := cmd.Output()
	if err != nil {
		return 0.0
	}
	var cpu float64
	fmt.Sscanf(strings.TrimSpace(string(out)), "%f", &cpu)
	return cpu
}

func getRAMUsage() float64 {
	cmd := exec.Command("sh", "-c", "free | grep Mem | awk '{print ($3/$2) * 100.0}'")
	out, err := cmd.Output()
	if err != nil {
		return 0.0
	}
	var ram float64
	fmt.Sscanf(strings.TrimSpace(string(out)), "%f", &ram)
	return ram
}

func getDiskUsage() float64 {
	cmd := exec.Command("sh", "-c", "df / | tail -1 | awk '{print $5}' | tr -d '%'")
	out, err := cmd.Output()
	if err != nil {
		return 0.0
	}
	var disk float64
	fmt.Sscanf(strings.TrimSpace(string(out)), "%f", &disk)
	return disk
}
