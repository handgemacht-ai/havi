package annotationmcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const EnvNoAutoRevive = "HAVI_NO_AUTO_REVIVE"

const daemonChildEnv = "HAVI_DAEMON_CHILD"

func serverBaseURL() string {
	host := os.Getenv("HAVI_HOST")
	if host == "" {
		host = "localhost"
	}
	port := os.Getenv("HAVI_PORT")
	if port == "" {
		port = os.Getenv("SERVER_PORT")
	}
	if port == "" {
		port = "8090"
	}
	return "http://" + host + ":" + port
}

func serverPort() string {
	port := os.Getenv("HAVI_PORT")
	if port == "" {
		port = os.Getenv("SERVER_PORT")
	}
	if port == "" {
		port = "8090"
	}
	return port
}

func bridgeDataDir() string {
	if d := os.Getenv("HAVI_DATA_DIR"); d != "" {
		return d
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "."
	}
	return filepath.Join(home, ".havi")
}

func pidAlive(pidFile string) (bool, int) {
	data, err := os.ReadFile(pidFile)
	if err != nil {
		return false, 0
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return false, 0
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false, pid
	}
	if err := proc.Signal(syscall.Signal(0)); err != nil {
		return false, pid
	}
	return true, pid
}

func spawnBridgeDaemon() error {
	dir := bridgeDataDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}

	pidFile := filepath.Join(dir, "havi.pid")
	if running, pid := pidAlive(pidFile); running {
		_ = pid
		return nil
	}

	logPath := filepath.Join(dir, "server.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open log %s: %w", logPath, err)
	}
	defer func() { _ = logFile.Close() }()

	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("executable path: %w", err)
	}

	cmd := exec.Command(exe, "serve")
	cmd.Env = append(os.Environ(),
		daemonChildEnv+"=1",
		"HAVI_PID_FILE="+pidFile,
	)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.Stdin = nil
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start child: %w", err)
	}

	if err := os.WriteFile(pidFile, []byte(strconv.Itoa(cmd.Process.Pid)), 0o644); err != nil {
		return fmt.Errorf("write pid file: %w", err)
	}

	return nil
}

func probeHealth(baseURL string, timeout time.Duration) bool {
	client := &http.Client{Timeout: timeout}
	resp, err := client.Get(baseURL + "/health")
	if err != nil {
		return false
	}
	_ = resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func waitForHealth(baseURL string, total time.Duration) bool {
	deadline := time.Now().Add(total)
	for time.Now().Before(deadline) {
		if probeHealth(baseURL, 300*time.Millisecond) {
			return true
		}
		time.Sleep(200 * time.Millisecond)
	}
	return false
}

func jsonRPCError(id any, code int, message string) []byte {
	type errObj struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	type errResp struct {
		Jsonrpc string `json:"jsonrpc"`
		ID      any    `json:"id"`
		Error   errObj `json:"error"`
	}
	b, _ := json.Marshal(errResp{
		Jsonrpc: "2.0",
		ID:      id,
		Error:   errObj{Code: code, Message: message},
	})
	return b
}

func extractIDFromFrame(frame []byte) any {
	var m map[string]any
	if err := json.Unmarshal(frame, &m); err != nil {
		return nil
	}
	return m["id"]
}

func parseSSEResponse(body []byte) ([][]byte, error) {
	var results [][]byte
	scanner := bufio.NewScanner(bytes.NewReader(body))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")
			results = append(results, []byte(data))
		}
	}
	return results, scanner.Err()
}

func Run(ctx context.Context, in io.Reader, out io.Writer) error {
	baseURL := serverBaseURL()
	client := &http.Client{Timeout: 30 * time.Second}

	var sessionID string
	serverReady := false
	noAutoRevive := os.Getenv(EnvNoAutoRevive) != ""

	scanner := bufio.NewScanner(in)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}

		frameID := extractIDFromFrame(line)

		if !serverReady {
			if probeHealth(baseURL, 200*time.Millisecond) {
				serverReady = true
			} else if noAutoRevive {
				port := serverPort()
				errMsg := fmt.Sprintf("havi server is not running on port %s — start it manually with: havi serve", port)
				errLine := append(jsonRPCError(frameID, -32000, errMsg), '\n')
				_, _ = out.Write(errLine)
				continue
			} else {
				if err := spawnBridgeDaemon(); err != nil {
					errMsg := fmt.Sprintf("failed to spawn havi server: %v", err)
					errLine := append(jsonRPCError(frameID, -32000, errMsg), '\n')
					_, _ = out.Write(errLine)
					continue
				}
				if !waitForHealth(baseURL, 5*time.Second) {
					port := serverPort()
					errMsg := fmt.Sprintf("havi server did not become ready on port %s within 5s", port)
					errLine := append(jsonRPCError(frameID, -32000, errMsg), '\n')
					_, _ = out.Write(errLine)
					continue
				}
				serverReady = true
			}
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/mcp", bytes.NewReader(line))
		if err != nil {
			errLine := append(jsonRPCError(frameID, -32603, fmt.Sprintf("build request: %v", err)), '\n')
			_, _ = out.Write(errLine)
			continue
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")
		if sessionID != "" {
			req.Header.Set("Mcp-Session-Id", sessionID)
		}

		resp, err := client.Do(req)
		if err != nil {
			serverReady = false
			sessionID = ""
			errLine := append(jsonRPCError(frameID, -32000, fmt.Sprintf("server unreachable: %v", err)), '\n')
			_, _ = out.Write(errLine)
			continue
		}

		if sid := resp.Header.Get("Mcp-Session-Id"); sid != "" {
			sessionID = sid
		}

		body, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			errLine := append(jsonRPCError(frameID, -32603, fmt.Sprintf("read response: %v", err)), '\n')
			_, _ = out.Write(errLine)
			continue
		}

		frames, err := parseSSEResponse(body)
		if err != nil {
			errLine := append(jsonRPCError(frameID, -32603, fmt.Sprintf("parse SSE: %v", err)), '\n')
			_, _ = out.Write(errLine)
			continue
		}

		for _, f := range frames {
			_, _ = out.Write(append(f, '\n'))
		}
	}

	if sessionID != "" {
		delReq, err := http.NewRequestWithContext(ctx, http.MethodDelete, baseURL+"/mcp", nil)
		if err == nil {
			delReq.Header.Set("Mcp-Session-Id", sessionID)
			resp, err := client.Do(delReq)
			if err == nil {
				_ = resp.Body.Close()
			}
		}
	}

	return nil
}
