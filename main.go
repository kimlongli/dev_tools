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

type DiyToolGroup struct {
	GroupName string    `json:"group_name"`
	Tools     []DiyTool `json:"tools"`
}

type DiyToolWithGroup struct {
	GroupName string `json:"group_name,omitempty"`
	DiyTool
}

var (
	snapshots     Snapshots
	snapshotsFile = "./snapshots.json"
	diyTools      []DiyToolWithGroup
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
	diyTools = []DiyToolWithGroup{}

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

		// 尝试解析为工具组格式
		var toolGroup DiyToolGroup
		if err := json.Unmarshal(data, &toolGroup); err == nil && len(toolGroup.Tools) > 0 {
			for _, t := range toolGroup.Tools {
				diyTools = append(diyTools, DiyToolWithGroup{
					GroupName: toolGroup.GroupName,
					DiyTool:   t,
				})
			}
			continue
		}

		// 尝试解析为单个工具对象（向后兼容）
		var tool DiyTool
		if err := json.Unmarshal(data, &tool); err == nil && tool.Name != "" {
			diyTools = append(diyTools, DiyToolWithGroup{DiyTool: tool})
		}
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

type CsvDiffRequest struct {
	OldContent string `json:"old_content"`
	NewContent string `json:"new_content"`
}

type CellDiff struct {
	Type int    `json:"type"`
	Line string `json:"line"`
}

type RowDiff struct {
	OrigIndex int        `json:"orig_index"`
	NewIndex  int        `json:"new_index"`
	Status    string     `json:"status"`
	Cells     []CellDiff `json:"cells,omitempty"`
}

type ColumnMapping struct {
	OrigIndex int    `json:"orig_index"`
	NewIndex  int    `json:"new_index"`
	Status    string `json:"status"`
}

type CsvDiffResponse struct {
	Columns []ColumnMapping `json:"columns"`
	Rows    []RowDiff       `json:"rows"`
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
	mux.HandleFunc("/api/home-dir", handleHomeDir)
	mux.HandleFunc("/api/csv-diff", handleCsvDiff)

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

func handleHomeDir(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	home, err := os.UserHomeDir()
	if err != nil {
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	json.NewEncoder(w).Encode(map[string]string{"home": home})
}

func handleCsvDiff(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req CsvDiffRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	origRows := parseCSVRows(req.OldContent)
	modRows := parseCSVRows(req.NewContent)

	origHeaders := []string{}
	modHeaders := []string{}
	if len(origRows) > 0 {
		origHeaders = origRows[0]
	}
	if len(modRows) > 0 {
		modHeaders = modRows[0]
	}

	columns := matchColumnsLCS(origHeaders, modHeaders)

	var rows []RowDiff
	if len(origRows) > 0 && len(modRows) > 0 {
		origDataRows := origRows[1:]
		modDataRows := modRows[1:]
		rows = matchRowsLCS(origDataRows, modDataRows, columns)
	}

	json.NewEncoder(w).Encode(CsvDiffResponse{
		Columns: columns,
		Rows:    rows,
	})
}

func parseCSVRows(text string) [][]string {
	rows := [][]string{}
	currentRow := []string{}
	currentCell := ""
	inQuotes := false

	for i := 0; i < len(text); i++ {
		ch := text[i]

		if ch == '"' {
			if inQuotes && i+1 < len(text) && text[i+1] == '"' {
				currentCell += "\""
				i++
			} else {
				inQuotes = !inQuotes
			}
		} else if ch == ',' && !inQuotes {
			currentRow = append(currentRow, currentCell)
			currentCell = ""
		} else if (ch == '\n' || ch == '\r') && !inQuotes {
			currentRow = append(currentRow, currentCell)
			rows = append(rows, currentRow)
			currentRow = []string{}
			currentCell = ""
			if ch == '\r' && i+1 < len(text) && text[i+1] == '\n' {
				i++
			}
		} else {
			currentCell += string(ch)
		}
	}
	if currentCell != "" || len(currentRow) > 0 {
		currentRow = append(currentRow, currentCell)
		rows = append(rows, currentRow)
	}

	return rows
}

func matchColumnsLCS(orig, mod []string) []ColumnMapping {
	m, n := len(orig), len(mod)
	dp := make([][]int, m+1)
	for i := range dp {
		dp[i] = make([]int, n+1)
	}

	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			if orig[i-1] == mod[j-1] {
				dp[i][j] = dp[i-1][j-1] + 1
			} else {
				if dp[i-1][j] > dp[i][j-1] {
					dp[i][j] = dp[i-1][j]
				} else {
					dp[i][j] = dp[i][j-1]
				}
			}
		}
	}

	result := []ColumnMapping{}
	i, j := m, n
	for i > 0 || j > 0 {
		if i > 0 && j > 0 && orig[i-1] == mod[j-1] {
			result = append([]ColumnMapping{{OrigIndex: i - 1, NewIndex: j - 1, Status: "same"}}, result...)
			i--
			j--
		} else if i > 0 && (j == 0 || dp[i-1][j] >= dp[i][j-1]) {
			result = append([]ColumnMapping{{OrigIndex: i - 1, NewIndex: -1, Status: "removed"}}, result...)
			i--
		} else if j > 0 {
			result = append([]ColumnMapping{{OrigIndex: -1, NewIndex: j - 1, Status: "added"}}, result...)
			j--
		}
	}

	return result
}

func matchRowsLCS(orig, mod [][]string, columns []ColumnMapping) []RowDiff {
	m, n := len(orig), len(mod)

	cellScore := make([][]int, m+1)
	for i := range cellScore {
		cellScore[i] = make([]int, n+1)
	}

	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			if rowsEqual(orig[i-1], mod[j-1]) {
				cellScore[i][j] = cellScore[i-1][j-1] + len(orig[i-1])
			} else {
				matchCount := countMatchingColumns(orig[i-1], mod[j-1], columns)
				if matchCount > 0 {
					cellScore[i][j] = cellScore[i-1][j-1] + matchCount
				} else if cellScore[i-1][j] > cellScore[i][j-1] {
					cellScore[i][j] = cellScore[i-1][j]
				} else {
					cellScore[i][j] = cellScore[i][j-1]
				}
			}
		}
	}

	result := []RowDiff{}
	i, j := m, n
	for i > 0 || j > 0 {
		if i > 0 && j > 0 && i-1 == j-1 && rowsEqualBySameColumns(orig[i-1], mod[j-1], columns) {
			if rowsEqual(orig[i-1], mod[j-1]) {
				result = append([]RowDiff{{OrigIndex: i - 1, NewIndex: j - 1, Status: "same"}}, result...)
			} else {
				result = append([]RowDiff{{OrigIndex: i - 1, NewIndex: j - 1, Status: "changed"}}, result...)
			}
			i--
			j--
		} else if i > 0 && j > 0 && rowsEqual(orig[i-1], mod[j-1]) {
			result = append([]RowDiff{{OrigIndex: i - 1, NewIndex: j - 1, Status: "same"}}, result...)
			i--
			j--
		} else if i > 0 && j > 0 && rowsEqualBySameColumns(orig[i-1], mod[j-1], columns) {
			result = append([]RowDiff{{OrigIndex: i - 1, NewIndex: j - 1, Status: "changed"}}, result...)
			i--
			j--
		} else if i > 0 && (j == 0 || cellScore[i-1][j] >= cellScore[i][j-1]) {
			result = append([]RowDiff{{OrigIndex: i - 1, NewIndex: -1, Status: "removed"}}, result...)
			i--
		} else if j > 0 {
			result = append([]RowDiff{{OrigIndex: -1, NewIndex: j - 1, Status: "added"}}, result...)
			j--
		}
	}

	for idx := range result {
		r := &result[idx]
		if r.Status == "same" {
			r.Cells = []CellDiff{}
		} else if r.Status == "changed" {
			if r.OrigIndex >= 0 && r.NewIndex >= 0 {
				r.Cells = computeRowCellDiff(orig[r.OrigIndex], mod[r.NewIndex], columns)
			}
		}
	}

	return result
}

func rowsEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func rowsEqualBySameColumns(a, b []string, columns []ColumnMapping) bool {
	sameCols := []struct{ origIdx, modIdx int }{}
	for _, col := range columns {
		if col.Status == "same" {
			sameCols = append(sameCols, struct{ origIdx, modIdx int }{col.OrigIndex, col.NewIndex})
		}
	}

	if len(sameCols) == 0 {
		return false
	}

	matchCount := 0
	for _, col := range sameCols {
		origVal := ""
		modVal := ""
		if col.origIdx >= 0 && col.origIdx < len(a) {
			origVal = a[col.origIdx]
		}
		if col.modIdx >= 0 && col.modIdx < len(b) {
			modVal = b[col.modIdx]
		}
		if origVal == modVal {
			matchCount++
		}
	}
	return matchCount >= 1
}

func countMatchingColumns(a, b []string, columns []ColumnMapping) int {
	count := 0
	for _, col := range columns {
		if col.Status == "same" {
			origVal := ""
			modVal := ""
			if col.OrigIndex >= 0 && col.OrigIndex < len(a) {
				origVal = a[col.OrigIndex]
			}
			if col.NewIndex >= 0 && col.NewIndex < len(b) {
				modVal = b[col.NewIndex]
			}
			if origVal == modVal {
				count++
			}
		}
	}
	return count
}

func rowLCSLen(a, b []string) int {
	m, n := len(a), len(b)
	dp := make([][]int, m+1)
	for i := range dp {
		dp[i] = make([]int, n+1)
	}
	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			if a[i-1] == b[j-1] {
				dp[i][j] = dp[i-1][j-1] + 1
			} else {
				if dp[i-1][j] > dp[i][j-1] {
					dp[i][j] = dp[i-1][j]
				} else {
					dp[i][j] = dp[i][j-1]
				}
			}
		}
	}
	return dp[m][n]
}

func computeRowCellDiff(origRow, modRow []string, columns []ColumnMapping) []CellDiff {
	sameCols := []struct{ origIdx, modIdx int }{}
	removedCols := []int{}
	addedCols := []int{}

	for _, col := range columns {
		if col.Status == "removed" {
			removedCols = append(removedCols, col.OrigIndex)
		} else if col.Status == "added" {
			addedCols = append(addedCols, col.NewIndex)
		} else {
			sameCols = append(sameCols, struct{ origIdx, modIdx int }{col.OrigIndex, col.NewIndex})
		}
	}

	result := []CellDiff{}

	for _, col := range sameCols {
		origVal := ""
		modVal := ""
		if col.origIdx >= 0 && col.origIdx < len(origRow) {
			origVal = origRow[col.origIdx]
		}
		if col.modIdx >= 0 && col.modIdx < len(modRow) {
			modVal = modRow[col.modIdx]
		}
		if origVal == modVal {
			result = append(result, CellDiff{Type: 0, Line: origVal})
		} else {
			result = append(result, CellDiff{Type: 1, Line: modVal})
			result = append(result, CellDiff{Type: -1, Line: origVal})
		}
	}

	for _, idx := range removedCols {
		origVal := ""
		if idx >= 0 && idx < len(origRow) {
			origVal = origRow[idx]
		}
		result = append(result, CellDiff{Type: -1, Line: origVal})
	}

	for _, idx := range addedCols {
		modVal := ""
		if idx >= 0 && idx < len(modRow) {
			modVal = modRow[idx]
		}
		result = append(result, CellDiff{Type: 1, Line: modVal})
	}

	return result
}
