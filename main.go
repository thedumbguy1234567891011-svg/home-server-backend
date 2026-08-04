package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"
)

// ServerConfig holds application-level settings like Safe Mode
type ServerConfig struct {
	SafeMode bool `json:"safe_mode"`
}

var config = ServerConfig{
	SafeMode: true,
}

func main() {
	mux := http.NewServeMux()

	// Register API endpoints
	mux.HandleFunc("/api/status/live", handleLiveStatus)
	mux.HandleFunc("/api/files", handleFiles)
	mux.HandleFunc("/api/files/delete", handleDeleteFile)
	mux.HandleFunc("/api/files/create-dir", handleCreateDir)
	mux.HandleFunc("/api/files/content", handleFileContent)
	mux.HandleFunc("/api/files/transfer", handleFileTransfer)
	mux.HandleFunc("/api/settings/safemode", handleToggleSafeMode)
	mux.HandleFunc("/api/settings/terms", handleTermsOfUse)
	mux.HandleFunc("/api/processes", handleProcesses)

	port := ":8080"
	fmt.Printf("[+] Home Server Backend running on port %s...\n", port)
	log.Fatal(http.ListenAndServe(port, mux))
}

// Helper to get OS Info
func getOSName() string {
	if runtime.GOOS == "linux" {
		data, err := os.ReadFile("/etc/os-release")
		if err == nil {
			lines := strings.Split(string(data), "\n")
			for _, line := range lines {
				if strings.HasPrefix(line, "PRETTY_NAME=") {
					parts := strings.Split(line, "=")
					if len(parts) > 1 {
						return strings.Trim(parts[1], "\"")
					}
				}
			}
		}
	}
	return runtime.GOOS
}

// Read actual CPU usage via /proc/stat delta
var prevIdle, prevTotal uint64

func getCPUUsage() float64 {
	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		return 0.0
	}
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "cpu ") {
			fields := strings.Fields(line)
			if len(fields) < 8 {
				return 0.0
			}
			var user, nice, system, idle, iowait, irq, softirq uint64
			fmt.Sscanf(fields[1], "%d", &user)
			fmt.Sscanf(fields[2], "%d", &nice)
			fmt.Sscanf(fields[3], "%d", &system)
			fmt.Sscanf(fields[4], "%d", &idle)
			fmt.Sscanf(fields[5], "%d", &iowait)
			fmt.Sscanf(fields[6], "%d", &irq)
			fmt.Sscanf(fields[7], "%d", &softirq)

			idleTotal := idle + iowait
			nonIdle := user + nice + system + irq + softirq
			total := idleTotal + nonIdle

			if prevTotal == 0 {
				prevIdle = idleTotal
				prevTotal = total
				return 2.0
			}

			totalDiff := total - prevTotal
			idleDiff := idleTotal - prevIdle

			prevIdle = idleTotal
			prevTotal = total

			if totalDiff == 0 {
				return 0.0
			}

			cpuUsage := float64(totalDiff-idleDiff) / float64(totalDiff) * 100.0
			if cpuUsage < 0 {
				return 0.0
			}
			return cpuUsage
		}
	}
	return 0.0
}

// Read actual RAM usage via /proc/meminfo
func getRAMUsage() float64 {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0.0
	}
	var totalMem, availMem uint64
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "MemTotal:") {
			fmt.Sscanf(line, "MemTotal: %d", &totalMem)
		} else if strings.HasPrefix(line, "MemAvailable:") {
			fmt.Sscanf(line, "MemAvailable: %d", &availMem)
		}
	}
	if totalMem == 0 {
		return 0.0
	}
	usedMem := totalMem - availMem
	return (float64(usedMem) / float64(totalMem)) * 100.0
}

// Read actual Disk usage of root partition '/'
func getDiskUsage() float64 {
	var stat syscall.Statfs_t
	err := syscall.Statfs("/", &stat)
	if err != nil {
		return 0.0
	}
	total := stat.Blocks * uint64(stat.Bsize)
	free := stat.Bavail * uint64(stat.Bsize)
	if total == 0 {
		return 0.0
	}
	used := total - free
	return (float64(used) / float64(total)) * 100.0
}

// 1. Live System Status (SSE)
func handleLiveStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	osName := getOSName()

	for {
		cpu := getCPUUsage()
		ram := getRAMUsage()
		disk := getDiskUsage()

		data := map[string]interface{}{
			"status":     "online",
			"os":         osName,
			"cpu_usage":  cpu,
			"ram_usage":  ram,
			"disk_usage": disk,
			"safe_mode":  config.SafeMode,
			"timestamp":  time.Now().Format(time.RFC3339),
		}
		jsonData, _ := json.Marshal(data)
		fmt.Fprintf(w, "data: %s\n\n", jsonData)
		
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		time.Sleep(2 * time.Second)
	}
}

// 2. File Explorer Listing
func handleFiles(w http.ResponseWriter, r *http.Request) {
	targetPath := r.URL.Query().Get("path")
	if targetPath == "" {
		targetPath = "/"
	}

	entries, err := os.ReadDir(targetPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	parentPath := filepath.Dir(targetPath)
	if targetPath == "/" {
		parentPath = ""
	}

	var fileList []map[string]interface{}
	for _, entry := range entries {
		info, err := entry.Info()
		sizeStr := ""
		if err == nil && !entry.IsDir() {
			sizeStr = fmt.Sprintf("%d bytes", info.Size())
		}
		fileList = append(fileList, map[string]interface{}{
			"name":   entry.Name(),
			"is_dir": entry.IsDir(),
			"size":   sizeStr,
		})
	}

	response := map[string]interface{}{
		"path":        targetPath,
		"parent_path": parentPath,
		"files":       fileList,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// 3. Path Protection Check
func isPathProtected(targetPath string) bool {
	clean := filepath.Clean(targetPath)
	criticalDirs := []string{"/boot", "/etc", "/bin", "/sbin", "/usr", "/lib", "/sys", "/proc", "/root"}
	
	for _, dir := range criticalDirs {
		if strings.HasPrefix(clean, dir) {
			return true
		}
	}
	return false
}

// 4. File Deletion with Safe Mode Enforcement
func handleDeleteFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete && r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	filePath := r.URL.Query().Get("path")
	if filePath == "" {
		http.Error(w, "Missing path parameter", http.StatusBadRequest)
		return
	}

	if config.SafeMode && isPathProtected(filePath) {
		http.Error(w, "Forbidden: Safe Mode is enabled. Deleting critical system files is restricted.", http.StatusForbidden)
		return
	}

	err := os.RemoveAll(filePath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"success","message":"path deleted"}`))
}

// 5. Create Directory
func handleCreateDir(w http.ResponseWriter, r *http.Request) {
	dirPath := r.URL.Query().Get("path")
	if dirPath == "" {
		http.Error(w, "Missing path", http.StatusBadRequest)
		return
	}
	err := os.MkdirAll(dirPath, 0755)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// 6. File Content Reader & Writer
func handleFileContent(w http.ResponseWriter, r *http.Request) {
	filePath := r.URL.Query().Get("path")
	if filePath == "" {
		http.Error(w, "Missing path", http.StatusBadRequest)
		return
	}

	if r.Method == http.MethodGet {
		content, err := os.ReadFile(filePath)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Write(content)
	} else if r.Method == http.MethodPost {
		if config.SafeMode && isPathProtected(filePath) {
			http.Error(w, "Forbidden: Safe Mode is enabled.", http.StatusForbidden)
			return
		}
		stat, _ := io.ReadAll(r.Body)
		err := os.WriteFile(filePath, stat, 0644)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}
}

// 7. Server-to-Server File Transfer Stub
func handleFileTransfer(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"transfer_initiated"}`))
}

// 8. Toggle Safe Mode Setting
func handleToggleSafeMode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	enabled := r.URL.Query().Get("enabled") == "true"
	config.SafeMode = enabled

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(config)
}

// 9. Terms of Use Endpoint
func handleTermsOfUse(w http.ResponseWriter, r *http.Request) {
	terms := map[string]string{
		"title": "Terms of Use & Disclaimer",
		"content": "This Home Server Control application provides remote access, file management, and system monitoring tools. " +
			"By disabling Safe Mode, you assume full responsibility for any modifications, deletions, or system instability caused " +
			"by remote file operations. The developers are not liable for data loss or system crashes resulting from administrative actions.",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(terms)
}

// 10. Live Processes Endpoint (Real Linux /proc parser)
func handleProcesses(w http.ResponseWriter, r *http.Request) {
	var processList []map[string]interface{}

	entries, err := os.ReadDir("/proc")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		isPid := true
		for _, ch := range name {
			if ch < '0' || ch > '9' {
				isPid = false
				break
			}
		}
		if !isPid {
			continue
		}

		commBytes, err := os.ReadFile(filepath.Join("/proc", name, "comm"))
		if err != nil {
			continue
		}
		procName := strings.TrimSpace(string(commBytes))

		statmBytes, err := os.ReadFile(filepath.Join("/proc", name, "statm"))
		var ramUsage float64 = 0.0
		if err == nil {
			fields := strings.Fields(string(statmBytes))
			if len(fields) > 0 {
				var pages uint64
				fmt.Sscanf(fields[0], "%d", &pages)
				ramUsage = (float64(pages * 4096) / (8 * 1024 * 1024 * 1024)) * 100.0
				if ramUsage > 100.0 {
					ramUsage = 100.0
				}
			}
		}

		processList = append(processList, map[string]interface{}{
			"name": procName,
			"cpu":  0.5,
			"ram":  float64(int(ramUsage*10))/10,
			"disk": 0.0,
		})

		if len(processList) >= 30 {
			break
		}
	}

	response := map[string]interface{}{
		"processes": processList,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
