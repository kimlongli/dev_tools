package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

type Snapshot struct {
	ID   int64                  `json:"id"`
	Name string                 `json:"name"`
	Time string                 `json:"time"`
	Data map[string]interface{} `json:"data"`
}

type Snapshots map[string][]Snapshot

type DiyField struct {
	FieldName string `json:"field_name"`
	FieldType string `json:"field_type"`
}

type DiyTool struct {
	Name   string     `json:"name"`
	Fields []DiyField `json:"fields"`
	Cmd    string     `json:"cmd"`
}

var (
	snapshots     Snapshots
	snapshotsFile = "./snapshots.json"
	diyTools      []DiyTool
	mu            sync.Mutex
)

func loadSnapshots() {
	mu.Lock()
	defer mu.Unlock()

	data, err := os.ReadFile(snapshotsFile)
	if err != nil {
		snapshots = make(Snapshots)
		return
	}

	if err := json.Unmarshal(data, &snapshots); err != nil {
		snapshots = make(Snapshots)
	}
}

func saveSnapshots() {
	data, err := json.MarshalIndent(snapshots, "", "  ")
	if err != nil {
		return
	}

	os.WriteFile(snapshotsFile, data, 0644)
}

func loadDiyTools() {
	diyTools = []DiyTool{}

	entries, err := os.ReadDir("./diy_tools")
	if err != nil {
		return
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		data, err := os.ReadFile("./diy_tools/" + entry.Name())
		if err != nil {
			continue
		}

		var tool DiyTool
		if err := json.Unmarshal(data, &tool); err != nil {
			continue
		}

		diyTools = append(diyTools, tool)
	}
}

type SnapshotRequest struct {
	Action string                 `json:"action"`
	Tool   string                 `json:"tool"`
	ID     int64                  `json:"id,omitempty"`
	Name   string                 `json:"name,omitempty"`
	Data   map[string]interface{} `json:"data,omitempty"`
}

type ExecRequest struct {
	Cmd  string   `json:"cmd"`
	Args []string `json:"args"`
}

type SaveFileRequest struct {
	Content  string `json:"content"`
	Filename string `json:"filename"`
}

type ReadFileRequest struct {
	Path string `json:"path"`
}

type ListDirRequest struct {
	Path string `json:"path"`
}

func main() {
	loadSnapshots()
	loadDiyTools()

	mux := http.NewServeMux()

	mux.HandleFunc("/api/snapshots", handleSnapshots)
	mux.HandleFunc("/api/diy-tools", handleDiyTools)
	mux.HandleFunc("/api/exec", handleExec)
	mux.HandleFunc("/api/save-file", handleSaveFile)
	mux.HandleFunc("/api/read-file", handleReadFile)
	mux.HandleFunc("/api/list-dir", handleListDir)

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.ServeFile(w, r, "./static/index.html")
			return
		}
		http.ServeFile(w, r, "./static/"+r.URL.Path)
	})

	fmt.Println("开发者工具箱已启动!")
	fmt.Println("请访问: http://localhost:29999")
	http.ListenAndServe(":29999", mux)
}

// 简化版文件服务器
type fileServer struct {
	root string
}

func NewFileServer(root string) *fileServer {
	return &fileServer{root: root}
}

func (fs *fileServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/" {
		r.URL.Path = "/index.html"
	}
	http.FileServer(http.Dir(fs.root)).ServeHTTP(w, r)
}

func handleSnapshots(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method == "GET" {
		mu.Lock()
		defer mu.Unlock()
		json.NewEncoder(w).Encode(snapshots)
		return
	}

	var req SnapshotRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	mu.Lock()
	defer mu.Unlock()

	switch req.Action {
	case "save":
		if req.Name == "" {
			req.Name = time.Now().Format("2006-01-02 15:04:05")
		}
		snapshot := Snapshot{
			ID:   time.Now().UnixMilli(),
			Name: req.Name,
			Time: time.Now().Format("2006-01-02 15:04:05"),
			Data: req.Data,
		}
		snapshots[req.Tool] = append(snapshots[req.Tool], snapshot)
		saveSnapshots()
		json.NewEncoder(w).Encode(map[string]interface{}{"id": snapshot.ID, "name": snapshot.Name, "time": snapshot.Time})

	case "delete":
		toolSnapshots := snapshots[req.Tool]
		for i, s := range toolSnapshots {
			if s.ID == req.ID {
				snapshots[req.Tool] = append(toolSnapshots[:i], toolSnapshots[i+1:]...)
				saveSnapshots()
				break
			}
		}
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})

	default:
		json.NewEncoder(w).Encode(map[string]string{"error": "unknown action"})
	}
}

func handleDiyTools(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	loadDiyTools()
	json.NewEncoder(w).Encode(diyTools)
}

func handleExec(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req ExecRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	if req.Cmd == "" {
		json.NewEncoder(w).Encode(map[string]string{"error": "cmd is empty"})
		return
	}

	args := append([]string{}, req.Args...)
	cmd := exec.Command(req.Cmd, args...)
	output, err := cmd.CombinedOutput()

	if err != nil {
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error(), "output": string(output)})
		return
	}

	json.NewEncoder(w).Encode(map[string]string{"output": string(output)})
}

func handleSaveFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req SaveFileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	filename := req.Filename
	if filename == "" {
		filename = "data.csv"
	}

	err := os.WriteFile(filename, []byte(req.Content), 0644)
	if err != nil {
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	json.NewEncoder(w).Encode(map[string]string{"status": "ok", "path": filename})
}

func handleReadFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ReadFileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	content, err := os.ReadFile(req.Path)
	if err != nil {
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"content": string(content),
		"path":    req.Path,
	})
}

func handleListDir(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ListDirRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	entries, err := os.ReadDir(req.Path)
	if err != nil {
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	type DirEntry struct {
		Name   string `json:"name"`
		IsDir  bool   `json:"isDir"`
		IsFile bool   `json:"isFile"`
	}

	var result []DirEntry
	for _, e := range entries {
		result = append(result, DirEntry{
			Name:   e.Name(),
			IsDir:  e.IsDir(),
			IsFile: !e.IsDir(),
		})
	}

	json.NewEncoder(w).Encode(result)
}
