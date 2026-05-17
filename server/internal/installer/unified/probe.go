package unified

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/tidwall/gjson"
)

// verifyJSONHaviKey returns a verifier that confirms the file at path
// contains a valid JSON document with the havi entry at the given gjson path
// (e.g. "mcpServers.havi" for Cursor, "servers.havi" for Copilot). The
// verifier is the smoke probe required by the epic — a writer that returned
// nil but produced a file the IDE cannot use is caught here.
func verifyJSONHaviKey(haviPath string) func(path string) error {
	return func(path string) error {
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read after write: %w", err)
		}
		entry := gjson.GetBytes(data, haviPath)
		if !entry.Exists() {
			return fmt.Errorf("%s missing after write", haviPath)
		}
		cmd := gjson.GetBytes(data, haviPath+".command").String()
		if cmd != "havi" {
			return fmt.Errorf("%s.command = %q, want \"havi\"", haviPath, cmd)
		}
		args := gjson.GetBytes(data, haviPath+".args").Array()
		if len(args) == 0 || args[0].String() != "mcp-bridge" {
			return fmt.Errorf("%s.args missing \"mcp-bridge\"", haviPath)
		}
		return nil
	}
}

// verifyCodexBlock confirms the codex managed block landed on disk and the
// inner [[mcp_servers]] entry contains the havi command.
func verifyCodexBlock(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read after write: %w", err)
	}
	body := string(data)
	if !containsAll(body, "havi-managed-block", "name = \"havi\"", "command = \"havi\"", "\"mcp-bridge\"") {
		return errors.New("codex managed block missing expected entries")
	}
	return nil
}

// verifyAgentsMDBlock confirms the AGENTS.md managed block exists and names
// the three MCP tools havi exposes.
func verifyAgentsMDBlock(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read after write: %w", err)
	}
	body := string(data)
	if !containsAll(body, "havi-managed-block", "list_annotations", "get_annotation_image", "resolve_annotation") {
		return errors.New("agents-md managed block missing expected tool references")
	}
	return nil
}

func containsAll(body string, needles ...string) bool {
	for _, n := range needles {
		if !strings.Contains(body, n) {
			return false
		}
	}
	return true
}

// serverHealth attempts to reach the local havi server's /health endpoint.
// Returns true with the resolved base URL when reachable. The result is
// informational only — it does not affect per-target install status (the
// daemon may not be running yet at install time and that's fine).
func serverHealth(ctx context.Context, port string) (string, bool) {
	if port == "" {
		port = "8090"
	}
	url := "http://localhost:" + port + "/health"
	ctx, cancel := context.WithTimeout(ctx, 1*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "http://localhost:" + port, false
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "http://localhost:" + port, false
	}
	defer resp.Body.Close()
	return "http://localhost:" + port, resp.StatusCode == http.StatusOK
}
