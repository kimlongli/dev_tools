# 开发者工具箱

一个轻量级的开发者工具箱，提供常用的开发辅助功能。

![示例截图](resources/example.png)

## 功能特性

### 1. JSON 格式化
- 支持 JSON 格式化和压缩
- 语法高亮显示

### 2. 时间戳转换
- 支持秒级和毫秒级时间戳
- 时间与时间戳互转
- 一键获取当前时间

### 3. JSON 提取
- 支持路径表达式提取 JSON 数据
- 支持 `[*]` 遍历数组
- 支持 `[n]` 获取指定索引
- 支持 `.key` 访问对象属性
- 提取结果支持 JSON 格式和逐行显示

示例：
- 输入 JSON：`[{"name": "test", "value": 1}, {"name": "test2", "value": 2}]`
- 表达式：`[*].value`
- 结果：`[1, 2]`

### 4. 文本比较器
- 智能 diff 算法，识别内容移动
- 绿色显示新增内容，红色显示删除内容

### 5. 自定义工具
- 支持在 `diy_tools` 目录添加自定义工具
- 每个工具通过 JSON 文件配置

## 自定义工具配置

在 `diy_tools` 目录下创建 JSON 文件，格式如下：

```json
{
    "name": "工具名称",
    "fields": [
        {
            "field_name": "参数名称",
            "field_type": "row"
        }
    ],
    "cmd": "/可执行程序路径"
}
```

### field_type 选项
- `text`：多行文本框（默认）
- `row`：单行输入框

## 快速开始

### 构建

```bash
go build -o devtools .
```

### 运行

```bash
./devtools
```

服务启动后访问 http://localhost:29999

## 项目结构

```
dev_tools/
├── main.go          # Go 后端代码
├── static/
│   └── index.html   # 前端页面
├── diy_tools/       # 自定义工具配置
├── resources/       # 资源文件
├── snapshots.json   # 快照数据（自动生成）
└── devtools        # 编译后的可执行文件
```

## 技术栈

- 后端：Go (标准库 net/http)
- 前端：原生 HTML5 + CSS3 + JavaScript
