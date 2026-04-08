package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
	OldContent string `json:"OldContent"`
	NewContent string `json:"NewContent"`
}

type TextDiffRequest struct {
	OldContent string `json:"old_content"`
	NewContent string `json:"new_content"`
}

type CharDiff struct {
	Type string `json:"type"` // "same", "space_added", "space_removed"
	Char string `json:"char"` // 字符（空白字符显示为符号）
}

type TextDiffLine struct {
	Type      string     `json:"type"`                 // "added", "removed", "unchanged"
	Value     string     `json:"value"`                // 原始行内容
	Special   bool       `json:"special,omitempty"`    // 是否为特殊行（仅空白字符差异）
	CharDiffs []CharDiff `json:"char_diffs,omitempty"` // 字符级diff（仅特殊行有）
}

type TextDiffResponse struct {
	Lines []TextDiffLine `json:"lines"`
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
	mux.HandleFunc("/api/text-diff", handleTextDiff)

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

// isWhitespace 检查字符是否为空白字符
func isWhitespace(c rune) bool {
	return c == ' ' || c == '\t' || c == '\r' || c == '\n' || c == '\f' || c == '\v'
}

// removeWhitespace 移除字符串中的所有空白字符
func removeWhitespace(s string) string {
	var result strings.Builder
	for _, c := range s {
		if !isWhitespace(c) {
			result.WriteRune(c)
		}
	}
	return result.String()
}

// tabToSpaces 将制表符转换为4个空格
func tabToSpaces(s string) string {
	return strings.ReplaceAll(s, "\t", "    ")
}

// isValidSpecialLine 检查两行是否适合作为特殊行（仅空白字符差异）
// 返回true如果两行适合作为特殊行匹配
func isValidSpecialLine(line1, line2 string) bool {
	// 通用文本diff基本检查：空行与非空行不匹配为特殊行
	if (line1 == "" && line2 != "") || (line1 != "" && line2 == "") {
		return false
	}

	// 检查是否一行只有空白字符而另一行不是
	// 例如："    "（只有空格）与"    }"不应匹配为特殊行
	if strings.TrimSpace(line1) == "" && strings.TrimSpace(line2) != "" {
		return false
	}
	if strings.TrimSpace(line1) != "" && strings.TrimSpace(line2) == "" {
		return false
	}

	// 检查前导空白视觉宽度差异
	// 如果两行的前导空白视觉宽度差异超过2个字符，则不应匹配为特殊行
	// 这可以防止不同缩进级别的行被错误匹配（如"    }"与"}"）
	leading1 := countLeadingWhitespaceVisual(line1)
	leading2 := countLeadingWhitespaceVisual(line2)
	if abs(leading1-leading2) > 2 {
		return false
	}

	// 通用文本diff：允许所有其他空白字符差异
	return true
}

// countLeadingWhitespace 计算字符串开头连续空白字符的数量
func countLeadingWhitespace(s string) int {
	count := 0
	for _, c := range s {
		if isWhitespace(c) {
			count++
		} else {
			break
		}
	}
	return count
}

// countLeadingWhitespaceVisual 计算字符串开头连续空白字符的视觉宽度
// 空格=1，制表符=4（标准制表位）
func countLeadingWhitespaceVisual(s string) int {
	width := 0
	for _, c := range s {
		if c == ' ' {
			width += 1
		} else if c == '\t' {
			width += 4
		} else if isWhitespace(c) {
			// 其他空白字符视为1宽度
			width += 1
		} else {
			break
		}
	}
	return width
}

// abs 返回整数的绝对值
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// whitespaceToSymbol 将空白字符转换为可见符号（不带填充）
func whitespaceToSymbol(c rune) string {
	switch c {
	case ' ':
		return "·"
	case '\t':
		return "→"
	case '\r':
		return "↵"
	case '\n':
		return "↵"
	case '\f':
		return "↓"
	case '\v':
		return "↕"
	default:
		return string(c)
	}
}

// getCharDisplay 根据字符类型和差异状态返回显示字符串
func getCharDisplay(c rune, diffType string) string {
	// diffType 可以是: "same", "space_added", "space_removed"
	isSpace := isWhitespace(c)
	isDiff := diffType == "space_added" || diffType == "space_removed"

	if !isSpace {
		// 非空白字符直接返回
		return string(c)
	}

	if !isDiff {
		// 相同的空白字符
		switch c {
		case ' ':
			return " "
		case '\t':
			// 相同制表符显示为4个空格
			return "    "
		case '\r', '\n':
			// 换行符不应该出现在行内字符diff中，但为了完整性
			return string(c)
		default:
			// 其他空白字符显示为1个空格
			return " "
		}
	} else {
		// 差异的空白字符
		switch c {
		case ' ':
			// 差异空格显示为符号，不填充
			return "·"
		case '\t':
			// 差异制表符显示为符号+3个空格，共4字符宽度
			return "→   "
		case '\r', '\n':
			return whitespaceToSymbol(c)
		default:
			return whitespaceToSymbol(c)
		}
	}
}

// compareLinesWithSpaceDiff 比较两行，检查是否为特殊行（仅空白字符差异）
// 返回 (是否为特殊行, 字符级diff)
func compareLinesWithSpaceDiff(line1, line2 string) (bool, []CharDiff) {
	// 去掉空白后比较
	if removeWhitespace(line1) != removeWhitespace(line2) {
		return false, nil
	}

	// 去掉空白后相同，检查原始行是否相同
	if line1 == line2 {
		// 完全相同的行，不是特殊行（因为没有任何差异）
		return false, nil
	}

	// 特殊行：仅空白字符差异，生成字符级diff
	chars1 := []rune(line1)
	chars2 := []rune(line2)
	m, n := len(chars1), len(chars2)

	// 使用LCS算法计算字符级diff，但将空白字符视为可互换
	dp := make([][]int, m+1)
	for i := range dp {
		dp[i] = make([]int, n+1)
		dp[i][0] = i
	}
	for j := 1; j <= n; j++ {
		dp[0][j] = j
	}

	// 同时记录操作类型：0=匹配，1=删除，2=添加，3=替换
	ops := make([][]int, m+1)
	for i := range ops {
		ops[i] = make([]int, n+1)
	}

	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			c1, c2 := chars1[i-1], chars2[j-1]

			// 检查是否匹配
			matched := false
			if c1 == c2 {
				matched = true
			} else if isWhitespace(c1) && isWhitespace(c2) {
				// 两个都是空白字符，但类型不同，视为替换
				matched = false
			}

			if matched {
				dp[i][j] = dp[i-1][j-1]
				ops[i][j] = 0 // 匹配
			} else {
				// 计算最小编辑距离
				delCost := dp[i-1][j] + 1
				addCost := dp[i][j-1] + 1
				// 替换成本：如果都是空白字符，成本为1（替换空白字符），否则成本为1（替换字符）
				subCost := dp[i-1][j-1] + 1

				minCost := delCost
				ops[i][j] = 1 // 删除

				if addCost < minCost {
					minCost = addCost
					ops[i][j] = 2 // 添加
				}
				if subCost < minCost {
					minCost = subCost
					ops[i][j] = 3 // 替换
				}

				dp[i][j] = minCost
			}
		}
	}

	// 回溯生成diff
	result := []CharDiff{}
	i, j := m, n
	for i > 0 || j > 0 {
		if i > 0 && j > 0 && ops[i][j] == 0 {
			// 匹配
			c := chars1[i-1]
			charType := "same"
			result = append([]CharDiff{{Type: charType, Char: getCharDisplay(c, charType)}}, result...)
			i--
			j--
		} else if j > 0 && (i == 0 || ops[i][j] == 2) {
			// 添加的字符
			c := chars2[j-1]
			charType := "same"
			if isWhitespace(c) {
				charType = "space_added"
			}
			result = append([]CharDiff{{Type: charType, Char: getCharDisplay(c, charType)}}, result...)
			j--
		} else if i > 0 && (j == 0 || ops[i][j] == 1) {
			// 删除的字符
			c := chars1[i-1]
			charType := "same"
			if isWhitespace(c) {
				charType = "space_removed"
			}
			result = append([]CharDiff{{Type: charType, Char: getCharDisplay(c, charType)}}, result...)
			i--
		} else if i > 0 && j > 0 && ops[i][j] == 3 {
			// 替换操作（字符不同）
			c1, c2 := chars1[i-1], chars2[j-1]
			// 由于我们知道去掉空白后内容相同，所以至少有一个是空白字符
			var type1, type2 string
			if isWhitespace(c1) {
				type1 = "space_removed"
			} else {
				type1 = "same"
			}
			if isWhitespace(c2) {
				type2 = "space_added"
			} else {
				type2 = "same"
			}
			result = append([]CharDiff{{Type: type1, Char: getCharDisplay(c1, type1)}}, result...)
			result = append([]CharDiff{{Type: type2, Char: getCharDisplay(c2, type2)}}, result...)
			i--
			j--
		} else {
			// 回退策略
			if i > 0 {
				c := chars1[i-1]
				charType := "same"
				if isWhitespace(c) {
					charType = "space_removed"
				}
				result = append([]CharDiff{{Type: charType, Char: getCharDisplay(c, charType)}}, result...)
				i--
			} else if j > 0 {
				c := chars2[j-1]
				charType := "same"
				if isWhitespace(c) {
					charType = "space_added"
				}
				result = append([]CharDiff{{Type: charType, Char: getCharDisplay(c, charType)}}, result...)
				j--
			}
		}
	}

	return true, result
}

func handleTextDiff(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req TextDiffRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	lines1 := strings.Split(req.OldContent, "\n")
	lines2 := strings.Split(req.NewContent, "\n")

	m := len(lines1)
	n := len(lines2)

	// 预计算行匹配信息
	// match[i][j]表示lines1[i-1]和lines2[j-1]的匹配类型：
	// 0: 不匹配
	// 1: 完全相同
	// 2: 特殊行（仅空白字符差异）
	match := make([][]int, m+1)
	for i := range match {
		match[i] = make([]int, n+1)
	}

	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			line1, line2 := lines1[i-1], lines2[j-1]
			if line1 == line2 {
				match[i][j] = 1 // 完全相同
			} else if removeWhitespace(line1) == removeWhitespace(line2) && isValidSpecialLine(line1, line2) {
				match[i][j] = 2 // 特殊行
			} else {
				match[i][j] = 0 // 不匹配
			}
		}
	}

	// 计算编辑距离矩阵（使用乘以2的因子：完全匹配=0，删除/添加=2，特殊行=2，替换=4）
	dp := make([][]int, m+1)
	for i := range dp {
		dp[i] = make([]int, n+1)
		dp[i][0] = i * 2 // 删除成本为2（乘以2后）
	}
	for j := 1; j <= n; j++ {
		dp[0][j] = j * 2 // 添加成本为2（乘以2后）
	}

	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			if match[i][j] == 1 {
				// 完全相同的行：成本0（乘以2后）
				dp[i][j] = dp[i-1][j-1]
			} else if match[i][j] == 2 {
				// 特殊行（仅空白字符差异）：成本2（乘以2后为4）
				dp[i][j] = dp[i-1][j-1] + 4
			} else {
				// 不匹配：计算最小编辑距离
				minVal := dp[i-1][j] + 2 // 删除成本2（乘以2后）
				if dp[i][j-1]+2 < minVal {
					minVal = dp[i][j-1] + 2 // 添加成本2（乘以2后）
				}
				if dp[i-1][j-1]+8 < minVal {
					minVal = dp[i-1][j-1] + 8 // 替换成本4（乘以2后为8）
				}
				dp[i][j] = minVal
			}
		}
	}

	// 回溯生成diff
	result := []TextDiffLine{}
	i, j := m, n
	for i > 0 || j > 0 {
		// 1. 完全相同的行（最高优先级）
		if i > 0 && j > 0 && match[i][j] == 1 && dp[i][j] == dp[i-1][j-1] {
			result = append([]TextDiffLine{{Type: "unchanged", Value: tabToSpaces(lines1[i-1])}}, result...)
			i--
			j--
			continue
		}

		// 2. 特殊行匹配（仅空白字符差异）
		if i > 0 && j > 0 && match[i][j] == 2 && dp[i][j] == dp[i-1][j-1]+4 {
			_, charDiffs := compareLinesWithSpaceDiff(lines1[i-1], lines2[j-1])
			result = append([]TextDiffLine{
				{
					Type:      "unchanged",
					Value:     tabToSpaces(lines1[i-1]),
					Special:   true,
					CharDiffs: charDiffs,
				},
			}, result...)
			i--
			j--
			continue
		}

		// 3. 删除操作
		if i > 0 && (j == 0 || dp[i][j] == dp[i-1][j]+2) {
			result = append([]TextDiffLine{{Type: "removed", Value: tabToSpaces(lines1[i-1])}}, result...)
			i--
			continue
		}

		// 4. 添加操作
		if j > 0 && (i == 0 || dp[i][j] == dp[i][j-1]+2) {
			result = append([]TextDiffLine{{Type: "added", Value: tabToSpaces(lines2[j-1])}}, result...)
			j--
			continue
		}

		// 5. 替换操作
		if i > 0 && j > 0 && dp[i][j] == dp[i-1][j-1]+8 {
			result = append([]TextDiffLine{{Type: "removed", Value: tabToSpaces(lines1[i-1])}}, result...)
			result = append([]TextDiffLine{{Type: "added", Value: tabToSpaces(lines2[j-1])}}, result...)
			i--
			j--
			continue
		}

		// 6. 回退策略：如果以上都不匹配，选择成本最小的操作
		if i > 0 && (j == 0 || dp[i-1][j] <= dp[i][j-1]) {
			// 删除
			result = append([]TextDiffLine{{Type: "removed", Value: tabToSpaces(lines1[i-1])}}, result...)
			i--
		} else if j > 0 {
			// 添加
			result = append([]TextDiffLine{{Type: "added", Value: tabToSpaces(lines2[j-1])}}, result...)
			j--
		}
	}

	json.NewEncoder(w).Encode(TextDiffResponse{Lines: result})
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

	type direction int
	const (
		diag direction = iota
		up
		left
	)

	cellScore := make([][]int, m+1)
	move := make([][]direction, m+1)
	for i := range cellScore {
		cellScore[i] = make([]int, n+1)
		move[i] = make([]direction, n+1)
	}

	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			if rowsEqual(orig[i-1], mod[j-1]) {
				cellScore[i][j] = cellScore[i-1][j-1] + len(orig[i-1])
				move[i][j] = diag
			} else {
				matchCount := countMatchingColumns(orig[i-1], mod[j-1], columns)
				if matchCount > 0 && cellScore[i-1][j-1]+matchCount >= cellScore[i-1][j] && cellScore[i-1][j-1]+matchCount >= cellScore[i][j-1] {
					cellScore[i][j] = cellScore[i-1][j-1] + matchCount
					move[i][j] = diag
				} else if cellScore[i-1][j] > cellScore[i][j-1] {
					cellScore[i][j] = cellScore[i-1][j]
					move[i][j] = up
				} else if cellScore[i][j-1] > cellScore[i-1][j] {
					cellScore[i][j] = cellScore[i][j-1]
					move[i][j] = left
				} else {
					cellScore[i][j] = cellScore[i-1][j]
					move[i][j] = up
				}
			}
		}
	}

	result := []RowDiff{}
	i, j := m, n
	for i > 0 || j > 0 {
		if i > 0 && j > 0 && move[i][j] == diag {
			if rowsEqual(orig[i-1], mod[j-1]) {
				result = append([]RowDiff{{OrigIndex: i - 1, NewIndex: j - 1, Status: "same"}}, result...)
			} else {
				result = append([]RowDiff{{OrigIndex: i - 1, NewIndex: j - 1, Status: "changed"}}, result...)
			}
			i--
			j--
		} else if i > 0 && (j == 0 || move[i][j] == up) {
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
