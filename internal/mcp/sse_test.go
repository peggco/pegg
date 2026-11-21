package mcp

import (
	"bufio"
	"context"
	json "github.com/goccy/go-json"
	"io"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"
)

type sseTestServer struct {
	authCheck string
	responses chan []byte
	gate      chan struct{}
}

func newSSETestServer(authCheck string) *sseTestServer {
	return &sseTestServer{authCheck: authCheck, responses: make(chan []byte, 8)}
}

func (s *sseTestServer) serve(ln net.Listener) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		go s.handle(conn)
	}
}

func (s *sseTestServer) handle(conn net.Conn) {
	defer conn.Close()

	r := bufio.NewReader(conn)
	head, err := readRequestHead(r)
	if err != nil {
		return
	}
	firstLine := strings.Split(head, "\r\n")[0]
	method := strings.Split(firstLine, " ")[0]
	headers := parseHeaders(head)

	if s.authCheck != "" && headers["authorization"] != s.authCheck {
		conn.Write([]byte("HTTP/1.1 401 Unauthorized\r\nContent-Length: 0\r\nConnection: close\r\n\r\n"))
		return
	}

	if method == "POST" {
		if s.gate != nil {
			<-s.gate
		}
		body := readRequestBody(r, headers)
		resp := makeResponse(body)
		s.responses <- resp
		// handle serves one request per connection and closes it on return, so
		// the response must say so: otherwise the client pools the connection
		// and the next POST races the close, failing with a connection reset.
		conn.Write([]byte("HTTP/1.1 202 Accepted\r\nContent-Length: 0\r\nConnection: close\r\n\r\n"))
		return
	}

	conn.Write([]byte("HTTP/1.1 200 OK\r\nContent-Type: text/event-stream\r\nConnection: keep-alive\r\n\r\n"))
	conn.Write([]byte("event: endpoint\ndata: /messages\n\n"))

	for {
		next := <-s.responses
		if _, err := conn.Write([]byte("event: message\ndata: " + string(next) + "\n\n")); err != nil {
			return
		}
	}
}

func readChan(ch chan []byte) ([]byte, bool) {
	select {
	case v, ok := <-ch:
		return v, ok
	}
}

func makeResponse(reqBody []byte) []byte {
	var req map[string]any
	_ = json.Unmarshal(reqBody, &req)
	method, _ := req["method"].(string)
	id, _ := req["id"].(float64)

	var result any
	switch method {
	case "initialize":
		result = map[string]any{
			"protocolVersion": "2024-11-05",
			"serverInfo":      map[string]any{"name": "sse-srv", "version": "1.0"},
		}
	case "tools/list":
		result = map[string]any{
			"tools": []map[string]any{
				{"name": "weather", "description": "get weather", "inputSchema": map[string]any{"type": "object"}},
			},
		}
	case "tools/call":
		params, _ := req["params"].(map[string]any)
		arguments, _ := params["arguments"].(map[string]any)
		city, _ := arguments["city"].(string)
		result = map[string]any{
			"content": []map[string]any{{"type": "text", "text": "weather in " + city}},
		}
	default:
		result = map[string]any{}
	}

	data, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      int(id),
		"result":  result,
	})
	return data
}

func parseHeaders(head string) map[string]string {
	out := make(map[string]string)
	lines := strings.Split(head, "\r\n")
	for _, line := range lines[1:] {
		if i := strings.IndexByte(line, ':'); i > 0 {
			out[strings.ToLower(strings.TrimSpace(line[:i]))] = strings.TrimSpace(line[i+1:])
		}
	}
	return out
}

func readRequestHead(r *bufio.Reader) (string, error) {
	var head []byte
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return "", err
		}
		head = append(head, line...)
		if line == "\r\n" || line == "\n" {
			break
		}
		if len(head) > 1<<20 {
			return "", io.EOF
		}
	}
	return string(head), nil
}

func readRequestBody(r *bufio.Reader, headers map[string]string) []byte {
	length := 0
	if v := headers["content-length"]; v != "" {
		length, _ = strconv.Atoi(v)
	}
	if length <= 0 {
		return nil
	}
	body := make([]byte, length)
	if _, err := io.ReadFull(r, body); err != nil {
		return nil
	}
	return body
}

func TestSSETransportFullFlow(t *testing.T) {
	srv := newSSETestServer("Bearer test-token")
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go srv.serve(ln)

	base := "http://" + ln.Addr().String()
	tr, err := NewSSETransport(SSEOptions{
		URL:     base + "/sse",
		Headers: map[string]string{"Authorization": "Bearer test-token"},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer tr.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := tr.Connect(ctx); err != nil {
		t.Fatal(err)
	}

	c := NewClient("sse-srv", tr, Options{Name: "sse-srv"})
	defer c.Close()

	info, err := c.Initialize(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if info.ServerInfo.Name != "sse-srv" {
		t.Fatalf("info = %+v", info)
	}

	tools, err := c.ListTools(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(tools) != 1 || tools[0].Name != "weather" {
		t.Fatalf("tools = %+v", tools)
	}

	res, err := c.CallTool(ctx, "weather", map[string]any{"city": "istanbul"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Content) != 1 || res.Content[0].Text != "weather in istanbul" {
		t.Fatalf("result = %+v", res)
	}
}

func TestSSETransportUnauthorized(t *testing.T) {
	srv := newSSETestServer("Bearer secret")
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go srv.serve(ln)

	tr, err := NewSSETransport(SSEOptions{
		URL:     "http://" + ln.Addr().String() + "/sse",
		Headers: map[string]string{"Authorization": "Bearer wrong"},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer tr.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := tr.Connect(ctx); err == nil {
		t.Fatal("expected connect to fail")
	}
}

func TestSSEParseLine(t *testing.T) {
	field, value := parseSSE([]byte("data: hello"))
	if field != "data" || value != "hello" {
		t.Fatalf("got %q=%q", field, value)
	}
	field, value = parseSSE([]byte("endpoint:/messages"))
	if field != "endpoint" || value != "/messages" {
		t.Fatalf("got %q=%q", field, value)
	}
	field, value = parseSSE([]byte("event:"))
	if field != "event" || value != "" {
		t.Fatalf("got %q=%q", field, value)
	}
	field, value = parseSSE([]byte("plain"))
	if field != "" || value != "plain" {
		t.Fatalf("plain line parsed as %q=%q", field, value)
	}
}
