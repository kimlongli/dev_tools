package main

import (
	"fmt"
	"net/http"
)

const html = `
<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>开发者工具箱</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', 'PingFang SC', 'Hiragino Sans GB', 'Microsoft YaHei', sans-serif; display: flex; height: 100vh; background: #1e1e1e; color: #d4d4d4; }
        .sidebar { width: 200px; background: #252526; border-right: 1px solid #3c3c3c; display: flex; flex-direction: column; }
        .sidebar-title { padding: 15px; font-size: 16px; font-weight: bold; border-bottom: 1px solid #3c3c3c; }
        .tool-item { padding: 12px 15px; cursor: pointer; border-bottom: 1px solid #2d2d2d; transition: background 0.2s; }
        .tool-item:hover { background: #2a2d2e; }
        .tool-item.active { background: #37373d; border-left: 3px solid #007acc; }
        .content { flex: 1; padding: 20px; overflow-y: auto; }
        .tool-panel { display: none; }
        .tool-panel.active { display: block; }
        .tool-title { font-size: 20px; margin-bottom: 20px; color: #fff; }
        label { display: block; margin-bottom: 8px; font-size: 14px; color: #9cdcfe; }
        input, textarea { width: 100%; padding: 10px; margin-bottom: 15px; background: #3c3c3c; border: 1px solid #3c3c3c; color: #d4d4d4; border-radius: 4px; font-size: 14px; }
        input:focus, textarea:focus { outline: none; border-color: #007acc; }
        textarea { height: 200px; font-family: 'Consolas', 'Monaco', monospace; resize: vertical; }
        button { padding: 10px 20px; background: #0e639c; color: #fff; border: none; border-radius: 4px; cursor: pointer; font-size: 14px; margin-right: 10px; margin-bottom: 10px; }
        button:hover { background: #1177bb; }
        .result { padding: 15px; background: #252526; border-radius: 4px; margin-top: 15px; white-space: pre-wrap; font-family: 'Consolas', 'Monaco', monospace; }
        .btn-group { margin-bottom: 15px; }
        #json-output { margin-bottom: 15px; }
        #json-output pre { margin: 0; padding: 15px; background: #282c34; border-radius: 4px; overflow: auto; max-height: 400px; }
        #json-output code { font-family: 'Consolas', 'Monaco', monospace; font-size: 14px; }
        .json-key { color: #e06c75; }
        .json-string { color: #98c379; }
        .json-number { color: #d19a66; }
        .json-boolean { color: #56b6c2; }
        .json-null { color: #c678dd; }
        .diff-container { display: flex; gap: 20px; }
        .diff-panel { flex: 1; }
        .diff-panel textarea { height: 300px; }
        .diff-output { margin-top: 15px; }
        .diff-output pre { margin: 0; padding: 15px; background: #282c34; border-radius: 4px; overflow: auto; max-height: 500px; font-family: 'Consolas', 'Monaco', monospace; font-size: 14px; line-height: 1.5; white-space: pre-wrap; }
        .diff-add { background: rgba(72, 199, 142, 0.2); color: #98c379; }
        .diff-remove { background: rgba(224, 108, 117, 0.2); color: #e06c75; }
        .diff-line { display: block; }
    </style>
</head>
<body>
    <div class="sidebar">
        <div class="sidebar-title">工具列表</div>
        <div class="tool-item active" data-tool="json">JSON 格式化</div>
        <div class="tool-item" data-tool="timestamp">时间戳转换</div>
        <div class="tool-item" data-tool="diff">文本比较器</div>
    </div>
    <div class="content">
        <div id="json" class="tool-panel active">
            <div class="tool-title">JSON 格式化</div>
            <label>输入 JSON:</label>
            <textarea id="json-input" placeholder='{"key": "value"}'></textarea>
            <button onclick="formatJSON()">格式化</button>
            <button onclick="minifyJSON()">压缩</button>
            <label>输出结果:</label>
            <div id="json-output"></div>
        </div>
        <div id="timestamp" class="tool-panel">
            <div class="tool-title">时间戳转换</div>
            <label>时间戳 (秒):</label>
            <input type="text" id="ts-input" placeholder="输入时间戳...">
            <label>时间 (格式: 2006-01-02 15:04:05):</label>
            <input type="text" id="dt-input" placeholder="输入时间...">
            <div class="btn-group">
                <button onclick="tsToTime()">时间戳转时间</button>
                <button onclick="timeToTs()">时间转时间戳</button>
                <button onclick="getNow()">获取当前时间</button>
            </div>
            <div id="ts-result" class="result"></div>
        </div>
        <div id="diff" class="tool-panel">
            <div class="tool-title">文本比较器</div>
            <div class="diff-container">
                <div class="diff-panel">
                    <label>原始文本:</label>
                    <textarea id="diff-original" placeholder="输入原始文本..."></textarea>
                </div>
                <div class="diff-panel">
                    <label>新文本:</label>
                    <textarea id="diff-new" placeholder="输入新文本..."></textarea>
                </div>
            </div>
            <button onclick="compareDiff()">比较</button>
            <button onclick="clearDiff()">清空</button>
            <div class="diff-output">
                <label>比较结果 (红色=删除, 绿色=新增):</label>
                <div id="diff-result"></div>
            </div>
        </div>
    </div>
    <script>
        document.querySelectorAll('.tool-item').forEach(item => {
            item.addEventListener('click', () => {
                document.querySelectorAll('.tool-item').forEach(i => i.classList.remove('active'));
                document.querySelectorAll('.tool-panel').forEach(p => p.classList.remove('active'));
                item.classList.add('active');
                document.getElementById(item.dataset.tool).classList.add('active');
            });
        });

        function highlightJSON(json) {
            json = json.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
            return json.replace(/("(\\u[a-zA-Z0-9]{4}|\\[^u]|[^\\"])*"(\s*:)?|\b(true|false|null)\b|-?\d+(?:\.\d*)?(?:[eE][+\-]?\d+)?)/g, function (match) {
                let cls = 'json-number';
                if (/^"/.test(match)) {
                    if (/:$/.test(match)) {
                        cls = 'json-key';
                    } else {
                        cls = 'json-string';
                    }
                } else if (/true|false/.test(match)) {
                    cls = 'json-boolean';
                } else if (/null/.test(match)) {
                    cls = 'json-null';
                }
                return '<span class="' + cls + '">' + match + '</span>';
            });
        }

        function formatJSON() {
            const input = document.getElementById('json-input').value;
            try {
                const obj = JSON.parse(input);
                const formatted = JSON.stringify(obj, null, 2);
                document.getElementById('json-output').innerHTML = '<pre><code>' + highlightJSON(formatted) + '</code></pre>';
            } catch(e) {
                document.getElementById('json-output').innerHTML = '<pre><code style="color: #e06c75;">错误: ' + e.message + '</code></pre>';
            }
        }

        function minifyJSON() {
            const input = document.getElementById('json-input').value;
            try {
                const obj = JSON.parse(input);
                const minified = JSON.stringify(obj);
                document.getElementById('json-output').innerHTML = '<pre><code>' + highlightJSON(minified) + '</code></pre>';
            } catch(e) {
                document.getElementById('json-output').innerHTML = '<pre><code style="color: #e06c75;">错误: ' + e.message + '</code></pre>';
            }
        }

        function tsToTime() {
            const ts = document.getElementById('ts-input').value;
            try {
                const timestamp = parseInt(ts);
                if (isNaN(timestamp)) throw new Error('无效的时间戳');
                const date = new Date(timestamp * 1000);
                const str = date.toLocaleString('zh-CN', { timeZone: 'Asia/Shanghai' });
                document.getElementById('dt-input').value = formatDate(date);
                document.getElementById('ts-result').textContent = '转换结果: ' + str;
            } catch(e) {
                document.getElementById('ts-result').textContent = '错误: ' + e.message;
            }
        }

        function timeToTs() {
            const dt = document.getElementById('dt-input').value;
            try {
                const date = new Date(dt);
                if (isNaN(date.getTime())) throw new Error('无效的时间格式');
                const ts = Math.floor(date.getTime() / 1000);
                document.getElementById('ts-input').value = ts;
                document.getElementById('ts-result').textContent = '转换结果: ' + ts;
            } catch(e) {
                document.getElementById('ts-result').textContent = '错误: ' + e.message;
            }
        }

        function getNow() {
            const now = new Date();
            const ts = Math.floor(now.getTime() / 1000);
            document.getElementById('ts-input').value = ts;
            document.getElementById('dt-input').value = formatDate(now);
            document.getElementById('ts-result').textContent = '当前时间戳: ' + ts + '\n当前时间: ' + now.toLocaleString('zh-CN', { timeZone: 'Asia/Shanghai' });
        }

        function formatDate(date) {
            const pad = n => n.toString().padStart(2, '0');
            return date.getFullYear() + '-' + pad(date.getMonth() + 1) + '-' + pad(date.getDate()) + ' ' + pad(date.getHours()) + ':' + pad(date.getMinutes()) + ':' + pad(date.getSeconds());
        }

        function diffLines(oldLines, newLines) {
            const result = [];
            const oldLen = oldLines.length;
            const newLen = newLines.length;
            
            // Simple LCS-based diff
            const dp = Array(oldLen + 1).fill(null).map(() => Array(newLen + 1).fill(0));
            for (let i = 1; i <= oldLen; i++) {
                for (let j = 1; j <= newLen; j++) {
                    if (oldLines[i-1] === newLines[j-1]) {
                        dp[i][j] = dp[i-1][j-1] + 1;
                    } else {
                        dp[i][j] = Math.max(dp[i-1][j], dp[i][j-1]);
                    }
                }
            }
            
            let i = oldLen, j = newLen;
            const diff = [];
            while (i > 0 || j > 0) {
                if (i > 0 && j > 0 && oldLines[i-1] === newLines[j-1]) {
                    diff.unshift({ type: 'equal', old: oldLines[i-1], new: newLines[j-1] });
                    i--; j--;
                } else if (j > 0 && (i === 0 || dp[i][j-1] >= dp[i-1][j])) {
                    diff.unshift({ type: 'add', new: newLines[j-1] });
                    j--;
                } else if (i > 0) {
                    diff.unshift({ type: 'remove', old: oldLines[i-1] });
                    i--;
                }
            }
            
            return diff;
        }

        function escapeHtml(str) {
            return str.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
        }

        function compareDiff() {
            const original = document.getElementById('diff-original').value;
            const newText = document.getElementById('diff-new').value;
            
            if (!original && !newText) {
                document.getElementById('diff-result').innerHTML = '<pre><code style="color: #e06c75;">请输入要比较的文本</code></pre>';
                return;
            }
            
            const oldLines = original.split('\n');
            const newLines = newText.split('\n');
            const diff = diffLines(oldLines, newLines);
            
            let html = '<pre><code>';
            let changes = 0;
            
            diff.forEach(d => {
                if (d.type === 'equal') {
                    html += '<span class="diff-line">' + escapeHtml(d.old) + '</span>';
                } else if (d.type === 'remove') {
                    html += '<span class="diff-line diff-remove">- ' + escapeHtml(d.old) + '</span>';
                    changes++;
                } else if (d.type === 'add') {
                    html += '<span class="diff-line diff-add">+ ' + escapeHtml(d.new) + '</span>';
                    changes++;
                }
            });
            
            html += '</code></pre>';
            
            if (changes === 0) {
                html = '<pre><code style="color: #98c379;">文本相同，无差异</code></pre>';
            } else {
                html += '<div style="margin-top: 10px; color: #9cdcfe;">共 ' + changes + ' 处差异</div>';
            }
            
            document.getElementById('diff-result').innerHTML = html;
        }

        function clearDiff() {
            document.getElementById('diff-original').value = '';
            document.getElementById('diff-new').value = '';
            document.getElementById('diff-result').innerHTML = '';
        }
    </script>
</body>
</html>
`

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(html))
	})

	fmt.Println("开发者工具箱已启动!")
	fmt.Println("请访问: http://localhost:8080")
	http.ListenAndServe(":8080", nil)
}
