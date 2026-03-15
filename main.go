package main

import (
	"fmt"
	"net/http"
)

func main() {
	fs := http.FileServer(http.Dir("./static"))
	http.Handle("/", fs)

	fmt.Println("开发者工具箱已启动!")
	fmt.Println("请访问: http://localhost:8080")
	http.ListenAndServe(":8080", nil)
}
