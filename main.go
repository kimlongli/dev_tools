package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

type HttpRequest struct {
	URL     string            `json:"url"`
	Method  string            `json:"method"`
	Headers map[string]string `json:"headers"`
	Body    string            `json:"body"`
}

type HttpResponse struct {
	Status     int               `json:"status"`
	StatusText string            `json:"statusText"`
	Headers    map[string]string `json:"headers"`
	Body       string            `json:"body"`
	Error      string            `json:"error,omitempty"`
	Message    string            `json:"message,omitempty"`
}

type GrpcRequest struct {
	Address string `json:"address"`
	Method  string `json:"method"`
	Body    string `json:"body"`
}

type GrpcResponse struct {
	Response string `json:"response,omitempty"`
	Error    string `json:"error,omitempty"`
	Message  string `json:"message,omitempty"`
}

func main() {
	fs := http.FileServer(http.Dir("./static"))
	http.Handle("/", fs)

	http.HandleFunc("/api/http", handleHttpRequest)
	http.HandleFunc("/api/grpc", handleGrpcRequest)

	fmt.Println("开发者工具箱已启动!")
	fmt.Println("请访问: http://localhost:8080")
	http.ListenAndServe(":8080", nil)
}

func handleHttpRequest(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req HttpRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		json.NewEncoder(w).Encode(HttpResponse{Error: "解析请求失败", Message: err.Error()})
		return
	}

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Timeout: 30 * time.Second, Transport: tr}
	httpReq, err := http.NewRequest(req.Method, req.URL, strings.NewReader(req.Body))
	if err != nil {
		json.NewEncoder(w).Encode(HttpResponse{Error: "创建请求失败", Message: err.Error()})
		return
	}

	for k, v := range req.Headers {
		httpReq.Header.Set(k, v)
	}

	resp, err := client.Do(httpReq)
	if err != nil {
		json.NewEncoder(w).Encode(HttpResponse{Error: "请求失败", Message: err.Error()})
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	respHeaders := make(map[string]string)
	for k, v := range resp.Header {
		respHeaders[k] = strings.Join(v, ", ")
	}

	json.NewEncoder(w).Encode(HttpResponse{
		Status:     resp.StatusCode,
		StatusText: resp.Status,
		Headers:    respHeaders,
		Body:       string(body),
	})
}

func handleGrpcRequest(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req GrpcRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		json.NewEncoder(w).Encode(GrpcResponse{Error: "解析请求失败", Message: err.Error()})
		return
	}

	if req.Address == "" || req.Method == "" {
		json.NewEncoder(w).Encode(GrpcResponse{Error: "请填写服务地址和方法"})
		return
	}

	conn, err := grpc.NewClient(req.Address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		json.NewEncoder(w).Encode(GrpcResponse{Error: "连接 gRPC 服务失败", Message: err.Error()})
		return
	}
	defer conn.Close()

	ctx, cancel := context.WithTimeout(metadata.NewOutgoingContext(context.Background(), metadata.New(nil)), 10*time.Second)
	defer cancel()

	var reqData []byte
	if req.Body != "" {
		reqData, _ = json.Marshal(req.Body)
	}

	var respData bytes.Buffer
	err = conn.Invoke(ctx, req.Method, bytes.NewReader(reqData), &respData)
	if err != nil {
		json.NewEncoder(w).Encode(GrpcResponse{Error: "gRPC 调用失败", Message: err.Error()})
		return
	}

	json.NewEncoder(w).Encode(GrpcResponse{Response: respData.String()})
}
