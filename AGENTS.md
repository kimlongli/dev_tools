# Developer Tools - AGENTS.md

This document provides guidelines for agents working on this codebase.

## Project Overview

- **Project name**: Developer Tools (开发者工具箱)
- **Language**: Go 1.21+
- **Type**: Web application (HTTP server with embedded HTML/CSS/JS)
- **Dependencies**: None (uses only Go standard library)

## Build Commands

```bash
# Build the application
go build -o devtools .

# Run the application
./devtools

# Run in development (auto-reload not available)
go run .
```

The server runs on `http://localhost:8080`

## Code Style Guidelines

### Go Code

1. **Imports**: Use standard library only. Group imports:
   ```go
   import (
       "fmt"
       "net/http"
   )
   ```

2. **Formatting**: Use `go fmt` for formatting

3. **Naming**:
   - Use camelCase for variables and functions
   - Use PascalCase for exported functions/types
   - Use descriptive names (e.g., `html` not `h`)

4. **Constants**: Use const for static strings
   ```go
   const html = `...`
   ```

5. **HTTP Handlers**: Keep inline for simple handlers

6. **Error Handling**: Use `fmt.Println` for logging errors in simple apps

### HTML/CSS/JS

1. **HTML Structure**: Use semantic HTML with meaningful class names

2. **CSS**:
   - Use dark theme colors (VS Code style)
   - Flexbox for layout
   - Use CSS variables for colors
   - Keep CSS in `<style>` tag in `<head>`

3. **JavaScript**:
   - Use vanilla JavaScript (no frameworks)
   - Use ES6+ syntax (const, arrow functions)
   - Event handling via addEventListener
   - Keep JS in `<script>` tag at end of `<body>`

4. **Naming**:
   - Use camelCase for functions and variables
   - Use descriptive IDs for elements
   - Class names: lowercase with hyphens (e.g., `.tool-item`)

## Project Structure

```
dev_tools/
├── main.go          # Main application (Go + embedded HTML/CSS/JS)
├── go.mod           # Go module file
└── devtools         # Compiled binary
```

## Adding New Tools

To add a new tool:

1. Add sidebar item in HTML:
   ```html
   <div class="tool-item" data-tool="toolname">Tool Name</div>
   ```

2. Add tool panel:
   ```html
   <div id="toolname" class="tool-panel">
       <!-- Tool content -->
   </div>
   ```

3. Add JavaScript functions for tool logic

4. Update sidebar click handler if needed

## Testing

This project has no automated tests. Manual testing:

1. Start server: `go run .` or `./devtools`
2. Open browser: `http://localhost:8080`
3. Test each tool:
   - JSON Formatter: Input valid/invalid JSON
   - Timestamp Converter: Test conversions
   - Diff Comparator: Compare two texts

## Common Tasks

### Running Single Test
N/A - No tests exist yet

### Linting
```bash
go vet .
```

### Adding Dependencies
```bash
go get <package>
```

## Technology Stack

- **Backend**: Go standard library (net/http)
- **Frontend**: Vanilla HTML5, CSS3, JavaScript (ES6+)
- **No external dependencies** for the web interface

## Notes

- The HTML/CSS/JS is embedded in Go as a string constant
- Server listens on port 8080 by default
- All tools run client-side (JavaScript)
- Chinese language is used for UI text
