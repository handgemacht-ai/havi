package annotationmcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

// --- SSE parser tests ---

func TestParseSSEResponse_SingleFrame(t *testing.T) {
	payload := `{"jsonrpc":"2.0","id":1,"result":{"tools":[]}}`
	input := "data: " + payload + "\n\n"

	frames, err := parseSSEResponse([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(frames) != 1 {
		t.Fatalf("expected 1 frame, got %d", len(frames))
	}
	if string(frames[0]) != payload {
		t.Errorf("frame = %q, want %q", string(frames[0]), payload)
	}
}

func TestParseSSEResponse_MultipleFrames(t *testing.T) {
	p1 := `{"jsonrpc":"2.0","id":1,"result":{}}`
	p2 := `{"jsonrpc":"2.0","id":2,"result":{}}`
	input := "data: " + p1 + "\n" + "data: " + p2 + "\n"

	frames, err := parseSSEResponse([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(frames) != 2 {
		t.Fatalf("expected 2 frames, got %d", len(frames))
	}
}

func TestParseSSEResponse_IgnoresNonDataLines(t *testing.T) {
	payload := `{"jsonrpc":"2.0","id":1,"result":{}}`
	input := "event: message\n" + "data: " + payload + "\n" + "\n"

	frames, err := parseSSEResponse([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(frames) != 1 {
		t.Fatalf("expected 1 frame, got %d", len(frames))
	}
	if string(frames[0]) != payload {
		t.Errorf("frame = %q, want %q", string(frames[0]), payload)
	}
}

func TestParseSSEResponse_Empty(t *testing.T) {
	frames, err := parseSSEResponse([]byte(""))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(frames) != 0 {
		t.Errorf("expected 0 frames, got %d", len(frames))
	}
}

// --- Session-ID capture and echo tests ---

func TestBridgeSessionIDCapture(t *testing.T) {
	wantSessionID := "test-session-abc"
	callCount := 0

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			callCount++
			if callCount == 1 {
				w.Header().Set("Mcp-Session-Id", wantSessionID)
			} else {
				got := r.Header.Get("Mcp-Session-Id")
				if got != wantSessionID {
					http.Error(w, fmt.Sprintf("bad session id: %q", got), http.StatusBadRequest)
					return
				}
			}
			w.Header().Set("Content-Type", "text/event-stream")
			resp := map[string]any{"jsonrpc": "2.0", "id": callCount, "result": map[string]any{}}
			b, _ := json.Marshal(resp)
			_, _ = fmt.Fprintf(w, "data: %s\n\n", b)
		case http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		case http.MethodGet:
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprint(w, `{"status":"ok"}`)
		}
	}))
	defer srv.Close()

	t.Setenv("HAVI_HOST", "")
	t.Setenv("SERVER_PORT", "")
	t.Setenv(EnvNoAutoRevive, "1")

	origGet := os.Getenv("HAVI_HOST")
	_ = origGet

	t.Setenv("HAVI_HOST", strings.TrimPrefix(strings.TrimPrefix(srv.URL, "http://"), "https://"))
	parts := strings.SplitN(strings.TrimPrefix(srv.URL, "http://"), ":", 2)
	if len(parts) == 2 {
		t.Setenv("HAVI_HOST", parts[0])
		t.Setenv("HAVI_PORT", parts[1])
	}

	frame1 := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}` + "\n"
	frame2 := `{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}` + "\n"

	input := frame1 + frame2
	var out bytes.Buffer

	ctx := context.Background()
	if err := Run(ctx, strings.NewReader(input), &out); err != nil {
		t.Fatalf("Run error: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) < 2 {
		t.Fatalf("expected at least 2 output lines, got %d: %q", len(lines), out.String())
	}

	if callCount < 2 {
		t.Errorf("expected at least 2 POST calls, got %d", callCount)
	}
}

// --- Opt-out env var check ---

func TestNoAutoReviveReturnsError(t *testing.T) {
	t.Setenv(EnvNoAutoRevive, "1")
	t.Setenv("HAVI_HOST", "127.0.0.1")
	t.Setenv("HAVI_PORT", "19999")

	frame := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}` + "\n"
	var out bytes.Buffer

	ctx := context.Background()
	if err := Run(ctx, strings.NewReader(frame), &out); err != nil {
		t.Fatalf("Run error: %v", err)
	}

	line := strings.TrimSpace(out.String())
	if line == "" {
		t.Fatal("expected error output, got empty")
	}

	var resp map[string]any
	if err := json.Unmarshal([]byte(line), &resp); err != nil {
		t.Fatalf("unmarshal error response: %v\noutput: %s", err, line)
	}
	if resp["error"] == nil {
		t.Errorf("expected error field in response, got: %v", resp)
	}
	errObj := resp["error"].(map[string]any)
	msg, _ := errObj["message"].(string)
	if !strings.Contains(msg, "19999") {
		t.Errorf("error message should mention port 19999, got: %q", msg)
	}
	if !strings.Contains(strings.ToLower(msg), "manually") {
		t.Errorf("error message should mention manual start, got: %q", msg)
	}
}

func TestNoAutoReviveEnvEmpty(t *testing.T) {
	t.Setenv(EnvNoAutoRevive, "")
	if os.Getenv(EnvNoAutoRevive) != "" {
		t.Fatal("env var should be empty")
	}
}

// --- Health probe test ---

func TestProbeHealth(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprint(w, `{"status":"ok"}`)
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	if !probeHealth(srv.URL, 500*time.Millisecond) {
		t.Error("expected health probe to succeed")
	}
	if probeHealth("http://127.0.0.1:19998", 100*time.Millisecond) {
		t.Error("expected health probe to fail for non-existent server")
	}
}

// --- Run with mocked server ---

func TestBridgeHappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/health":
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprint(w, `{"status":"ok"}`)
		case r.Method == http.MethodPost && r.URL.Path == "/mcp":
			w.Header().Set("Mcp-Session-Id", "sess-1")
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = fmt.Fprint(w, `data: {"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2025-03-26"}}`+"\n\n")
		case r.Method == http.MethodDelete && r.URL.Path == "/mcp":
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	defer srv.Close()

	host, port := parseHostPort(srv.URL)
	t.Setenv("HAVI_HOST", host)
	t.Setenv("HAVI_PORT", port)
	t.Setenv(EnvNoAutoRevive, "1")

	frame := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}` + "\n"
	var out bytes.Buffer

	if err := Run(context.Background(), strings.NewReader(frame), &out); err != nil {
		t.Fatalf("Run error: %v", err)
	}

	scanner := bufio.NewScanner(&out)
	var lines []string
	for scanner.Scan() {
		l := scanner.Text()
		if l != "" {
			lines = append(lines, l)
		}
	}
	if len(lines) == 0 {
		t.Fatal("expected at least one output line")
	}

	var resp map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &resp); err != nil {
		t.Fatalf("unmarshal: %v\nline: %s", err, lines[0])
	}
	if resp["result"] == nil {
		t.Errorf("expected result field, got: %v", resp)
	}
}

func TestBridgeClearsSessionIDOnTransportError(t *testing.T) {
	var (
		postCount   int32
		dead        bool
		seenHeaders []string
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/health":
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprint(w, `{"status":"ok"}`)
		case r.Method == http.MethodPost && r.URL.Path == "/mcp":
			postCount++
			seenHeaders = append(seenHeaders, r.Header.Get("Mcp-Session-Id"))
			if dead {
				panic(http.ErrAbortHandler)
			}
			sid := fmt.Sprintf("sess-%d", postCount)
			w.Header().Set("Mcp-Session-Id", sid)
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = fmt.Fprintf(w, "data: {\"jsonrpc\":\"2.0\",\"id\":%d,\"result\":{}}\n\n", postCount)
		case r.Method == http.MethodDelete && r.URL.Path == "/mcp":
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	defer srv.Close()

	host, port := parseHostPort(srv.URL)
	t.Setenv("HAVI_HOST", host)
	t.Setenv("HAVI_PORT", port)
	t.Setenv(EnvNoAutoRevive, "1")

	frame1 := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}` + "\n"
	frame2 := `{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}` + "\n"
	frame3 := `{"jsonrpc":"2.0","id":3,"method":"tools/list","params":{}}` + "\n"

	in := newScriptedReader([]string{
		frame1,
		"__kill__",
		frame2,
		"__revive__",
		frame3,
	}, func(tok string) {
		switch tok {
		case "__kill__":
			dead = true
		case "__revive__":
			dead = false
		}
	})

	var out bytes.Buffer
	if err := Run(context.Background(), in, &out); err != nil {
		t.Fatalf("Run error: %v", err)
	}

	if len(seenHeaders) != 3 {
		t.Fatalf("expected 3 POSTs, got %d (headers=%v)", len(seenHeaders), seenHeaders)
	}
	if seenHeaders[0] != "" {
		t.Errorf("frame 1 should have no session ID, got %q", seenHeaders[0])
	}
	if seenHeaders[1] != "sess-1" {
		t.Errorf("frame 2 should echo sess-1 (still believing daemon is alive), got %q", seenHeaders[1])
	}
	if seenHeaders[2] != "" {
		t.Errorf("frame 3 must NOT carry stale session after transport error, got %q", seenHeaders[2])
	}
}

type scriptedReader struct {
	frames []string
	hook   func(string)
	i      int
	cur    []byte
}

func newScriptedReader(frames []string, hook func(string)) *scriptedReader {
	return &scriptedReader{frames: frames, hook: hook}
}

func (r *scriptedReader) Read(p []byte) (int, error) {
	for len(r.cur) == 0 {
		if r.i >= len(r.frames) {
			return 0, fmt.Errorf("EOF")
		}
		f := r.frames[r.i]
		r.i++
		if strings.HasPrefix(f, "__") {
			r.hook(f)
			continue
		}
		r.cur = []byte(f)
	}
	n := copy(p, r.cur)
	r.cur = r.cur[n:]
	return n, nil
}

func parseHostPort(rawURL string) (string, string) {
	trimmed := strings.TrimPrefix(rawURL, "http://")
	parts := strings.SplitN(trimmed, ":", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return trimmed, "80"
}
