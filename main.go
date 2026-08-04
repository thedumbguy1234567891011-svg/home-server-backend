package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
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

type TransferRequest struct {
	TargetServerIP string `json:"target_server_ip"`
	SourcePath     string `json:"source_path"`
	DestinationDir string `json:"destination_dir"`
}

func main() {
	http.HandleFunc("/api/status/live", handleLiveStatus)
	http.HandleFunc("/api/files", handleFiles)
	http.HandleFunc("/api/files/create-dir", handleCreateDir)
	http.HandleFunc("/api/files/delete", handleDeleteFile)
	http.HandleFunc("/api/files/content", handleFileContent)
	http.HandleFunc("/api/files/upload", handleUploadFile)
	http.HandleFunc("/api/files/transfer", handleServerTransfer)

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

// 1. Create Folder
func handleCreateDir(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	dirPath := r.URL.Query().Get("path")
	if dirPath == "" {
		http.Error(w, "Path required", http.StatusBadRequest)
		return
	}
	err := os.MkdirAll(dirPath, 0755)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"success"}`))
}

// 2. Delete File or Folder
func handleDeleteFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete && r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	filePath := r.URL.Query().Get("path")
	err := os.RemoveAll(filePath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"deleted"}`))
}

// 3. Read or Modify File Content
func handleFileContent(w http.ResponseWriter, r *http.Request) {
	filePath := r.URL.Query().Get("path")
	if filePath == "" {
		http.Error(w, "Path required", http.StatusBadRequest)
		return
	}

	if r.Method == http.MethodGet {
		content, err := os.ReadFile(filePath)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/plain")
		w.Write(content)
	} else if r.Method == http.MethodPost {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		err = os.WriteFile(filePath, body, 0644)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"saved"}`))
	}
}

// 4. Upload File
func handleUploadFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	targetDir := r.URL.Query().Get("path")
	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer file.Close()

	dstPath := filepath.Join(targetDir, header.Filename)
	dst, err := os.Create(dstPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer dst.Close()

	_, err = io.Copy(dst, file)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"uploaded"}`))
}

// 5. Server-to-Server File Transfer Engine
func handleServerTransfer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req TransferRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Step A: Open source file locally
	srcFile, err := os.Open(req.SourcePath)
	if err != nil {
		http.Error(w, "Failed to open source file: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer srcFile.Close()

	// Step B: Prepare a pipe or multipart form request to stream it directly to the target server IP
	pr, pw := io.Pipe()
	defer pw.Close()

	// For simplicity in streaming via HTTP POST
	go func() {
		defer pw.Close()
		// We could implement direct streaming, or use standard multipart stream
	}();

	// Let's stream it using standard http post to target server endpoint /api/files/upload?path=destination_dir
	targetURL := fmt.Sprintf("http://%s/api/files/upload?path=%s", req.TargetServerIP, req.DestinationDir)
	
	// Create body reader using pipe
	pipeReader, pipeWriter := io.Pipe()
	writer := io.MultiWriter(pipeWriter)
	
	// We can use standard http client upload stream:
	// A robust way: Read file content and stream it over HTTP request
	reqBody, bodyWriter := io.Pipe()
	
	// We use multipart form writer to send the file smoothly
	// Or even simpler: Custom POST request piping the file directly
	_ = writer
	_ = pipeReader

	// Let's execute the direct push transfer:
	resp, err := http.Post(targetURL, "application/octet-stream", srcFile)
	if err != nil || resp.StatusCode != 200 {
		http.Error(w, "Transfer failed reaching target server", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"transfer_complete"}`))
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

func getCPUUsage() float64 {
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
