package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
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
	mux.HandleFunc("/api/files/delete", handleDeleteFile)
	mux.HandleFunc("/api/settings/safemode", handleToggleSafeMode)
	mux.HandleFunc("/api/settings/terms", handleTermsOfUse)

	port := ":8080"
	fmt.Printf("[+] Home Server Backend running on port %s...\n", port)
	log.Fatal(http.ListenAndServe(port, mux))
}

// 1. Live System Status (SSE)
func handleLiveStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	for {
		data := map[string]interface{}{
			"status":    "online",
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

// 2. Path Protection Check
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

// 3. File Deletion with Safe Mode Enforcement
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

	// Enforce Safe Mode restrictions
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

// 4. Toggle Safe Mode Setting
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

// 5. Terms of Use Endpoint (for mobile app settings menu display)
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
