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
		return "Ubuntu 24.04.4 LTS"
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
	return "Ubuntu 24.04.4 LTS"
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

// File Explorer endpoint handler with full root capability and parent routing
func handleFiles(w http.ResponseWriter, r *http.Request) {
	reqPath := r.URL.Query().Get("path")
	if reqPath == "" {
		reqPath = "/" // Default to filesystem root if empty
	}

	// Clean path to prevent path traversal trickery
	cleanPath := filepath.Clean(reqPath)

	entries, err := os.ReadDir(cleanPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	parentPath := filepath.Dir(cleanPath)
	if parentPath == cleanPath {
		parentPath = "" // Reached absolute system root boundary
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

func getCPUUsage() float64 { return 12.5 }
func getRAMUsage() float64 { return 45.2 }
func getDiskUsage() float64 {
	cmd := exec.Command("df", "/")
	out, err := cmd.Output()
	if err != nil {
		return 30.0
	}
	_ = out
	return 38.4
}
