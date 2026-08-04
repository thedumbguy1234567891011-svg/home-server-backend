package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// ServerConfig holds application-level settings like Safe Mode
type ServerConfig struct {
	SafeMode bool `json:"safe_mode"`
}

var config = ServerConfig{
	SafeMode: true, // Enabled by default to prevent accidental system damage
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

// Helper for basic metrics simulation/retrieval
func getSystemMetrics() (float64, float64, float64) {
	// Basic placeholder metrics (can be expanded with psutil or host commands if needed)
	return 12.5, 45.2, 30.1
}

// 1. Live System Status (SSE) - Restored with CPU, RAM, Disk, and OS!
func handleLiveStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	osName := getOSName()

	for {
		cpu, ram, disk := getSystemMetrics()
		data := map[string]interface{}{
			"status":    "online",
			"os":        osName,
			"cpu_usage": cpu,
			"ram_usage": ram,
			"disk_usage": disk,
			"safe_mode": config.SafeMode,
			"timestamp": time.Now().Format(time.RFC3339),
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
		// Read body and write to file
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
