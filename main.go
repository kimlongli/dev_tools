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

func main() {
	loadSnapshots()
	loadDiyTools()

	http.HandleFunc("/api/snapshots", handleSnapshots)
	http.HandleFunc("/api/diy-tools", handleDiyTools)
	http.HandleFunc("/api/exec", handleExec)

	fs := NewFileServer("./static")
	http.Handle("/", fs)

	fmt.Println("开发者工具箱已启动!")
	fmt.Println("请访问: http://localhost:29999")
	http.ListenAndServe(":29999", nil)
}

// 简化版文件服务器
type fileServer struct {
	root string
}

func NewFileServer(root string) *fileServer {
	return &fileServer{root: root}
}

func (fs *fileServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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
