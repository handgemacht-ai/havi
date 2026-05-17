// Package copilot writes and removes havi's MCP server entry in GitHub
// Copilot's mcp.json file. Like the cursor writer, the file is JSON, so we
// cannot use a textual managed block; instead we own one named key —
// servers.havi — and preserve every other key byte-for-byte via sjson's
// in-place value editing.
//
// Two write targets are supported:
//   - Project (default): .vscode/mcp.json in the current working directory.
//   - Global (--global): VS Code's user-profile mcp.json
//     (~/.config/Code/User/mcp.json on Linux,
//     ~/Library/Application Support/Code/User/mcp.json on macOS).
package copilot

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/tidwall/gjson"
	"github.com/tidwall/pretty"
	"github.com/tidwall/sjson"
)

// DetectCLI reports whether the VS Code CLI is present on PATH. GitHub Copilot
// is a VS Code extension, so `code --version` is the canonical IDE-present
// signal.
func DetectCLI() bool {
	bin, err := exec.LookPath("code")
	if err != nil {
		return false
	}
	cmd := exec.Command(bin, "--version")
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run() == nil
}

const haviEntry = `{"command":"havi","args":["mcp-bridge"]}`

const HaviPath = "servers.havi"

type Status int

const (
	StatusConfigured Status = iota
	StatusAlreadyConfigured
	StatusFailed
)

func (s Status) String() string {
	switch s {
	case StatusConfigured:
		return "configured"
	case StatusAlreadyConfigured:
		return "already-configured"
	default:
		return "failed"
	}
}

// ProjectPath returns ./.vscode/mcp.json unless HAVI_COPILOT_PROJECT_PATH
// overrides it.
func ProjectPath() (string, error) {
	if p := os.Getenv("HAVI_COPILOT_PROJECT_PATH"); p != "" {
		return p, nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("resolve cwd: %w", err)
	}
	return filepath.Join(cwd, ".vscode", "mcp.json"), nil
}

// GlobalPath returns the VS Code user-profile mcp.json path unless
// HAVI_COPILOT_GLOBAL_PATH overrides it.
func GlobalPath() (string, error) {
	if p := os.Getenv("HAVI_COPILOT_GLOBAL_PATH"); p != "" {
		return p, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home: %w", err)
	}
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", "Code", "User", "mcp.json"), nil
	default:
		return filepath.Join(home, ".config", "Code", "User", "mcp.json"), nil
	}
}

// Install adds or updates servers.havi in path.
func Install(path string) (Status, error) {
	current, wasFresh, err := readOrSeed(path)
	if err != nil {
		return StatusFailed, err
	}

	existing := gjson.GetBytes(current, HaviPath)
	if existing.Exists() && jsonEqual(existing.Raw, haviEntry) {
		return StatusAlreadyConfigured, nil
	}

	updated, err := sjson.SetRawBytes(current, HaviPath, []byte(haviEntry))
	if err != nil {
		return StatusFailed, fmt.Errorf("set %s: %w", HaviPath, err)
	}
	if wasFresh {
		updated = pretty.PrettyOptions(updated, &pretty.Options{
			Width:    80,
			Prefix:   "",
			Indent:   "  ",
			SortKeys: false,
		})
	}
	if err := writeAtomic(path, updated); err != nil {
		return StatusFailed, err
	}
	return StatusConfigured, nil
}

// Uninstall removes servers.havi from path. When removing havi makes the
// document represent "nothing havi added" the file is deleted so a fresh
// install + uninstall round-trip restores the pre-install missing-file state.
func Uninstall(path string) (Status, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return StatusAlreadyConfigured, nil
	}
	if err != nil {
		return StatusFailed, err
	}

	if !gjson.GetBytes(data, HaviPath).Exists() {
		return StatusAlreadyConfigured, nil
	}

	updated, err := sjson.DeleteBytes(data, HaviPath)
	if err != nil {
		return StatusFailed, fmt.Errorf("delete %s: %w", HaviPath, err)
	}

	if isEmptyConfig(updated) {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return StatusFailed, err
		}
		return StatusConfigured, nil
	}

	if err := writeAtomic(path, updated); err != nil {
		return StatusFailed, err
	}
	return StatusConfigured, nil
}

func readOrSeed(path string) ([]byte, bool, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return []byte("{}"), true, nil
	}
	if err != nil {
		return nil, false, err
	}
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("{}")) {
		return []byte("{}"), true, nil
	}
	if !json.Valid(data) {
		return nil, false, fmt.Errorf("parse %s: invalid JSON at %s", HaviPath, path)
	}
	return data, false, nil
}

func jsonEqual(a, b string) bool {
	var av, bv any
	if err := json.Unmarshal([]byte(a), &av); err != nil {
		return false
	}
	if err := json.Unmarshal([]byte(b), &bv); err != nil {
		return false
	}
	aBytes, err := json.Marshal(av)
	if err != nil {
		return false
	}
	bBytes, err := json.Marshal(bv)
	if err != nil {
		return false
	}
	return bytes.Equal(aBytes, bBytes)
}

// isEmptyConfig returns true when the document has no top-level keys other
// than servers, and servers itself has no remaining entries.
func isEmptyConfig(data []byte) bool {
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(data, &doc); err != nil {
		return false
	}
	if len(doc) == 0 {
		return true
	}
	if len(doc) != 1 {
		return false
	}
	servers, ok := doc["servers"]
	if !ok {
		return false
	}
	var inner map[string]json.RawMessage
	if err := json.Unmarshal(servers, &inner); err != nil {
		return false
	}
	return len(inner) == 0
}

func writeAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}
	tmp, err := os.CreateTemp(dir, ".mcp.json.*")
	if err != nil {
		return fmt.Errorf("create tempfile: %w", err)
	}
	tmpName := tmp.Name()
	cleanup := func() { _ = os.Remove(tmpName) }
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		cleanup()
		return fmt.Errorf("write tempfile: %w", err)
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return fmt.Errorf("close tempfile: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		cleanup()
		return fmt.Errorf("rename to %s: %w", path, err)
	}
	return nil
}
