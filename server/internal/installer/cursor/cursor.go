// Package cursor writes and removes havi's MCP server entry in
// ~/.cursor/mcp.json. The file is JSON, so we cannot use a textual managed
// block. Instead we own one named key — mcpServers.havi — and preserve every
// other key byte-for-byte via sjson's in-place value editing.
package cursor

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/tidwall/gjson"
	"github.com/tidwall/pretty"
	"github.com/tidwall/sjson"
)

// DetectCLI reports whether the Cursor CLI is present on PATH. Per the
// ide-detection-cli-only constraint, the IDE is "present" iff its CLI exits
// zero. We invoke `cursor --version` rather than relying on file paths.
func DetectCLI() bool {
	bin, err := exec.LookPath("cursor")
	if err != nil {
		return false
	}
	cmd := exec.Command(bin, "--version")
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run() == nil
}

// haviEntry is the JSON value havi writes under mcpServers.havi.
const haviEntry = `{"command":"havi","args":["mcp-bridge"]}`

// HaviPath is the JSONPath we own inside the mcp.json document.
const HaviPath = "mcpServers.havi"

// Status describes the outcome of an Install or Uninstall call.
type Status int

const (
	StatusConfigured        Status = iota // file was written
	StatusAlreadyConfigured               // current state already matched target
	StatusFailed                          // error encountered
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

// ConfigPath returns ~/.cursor/mcp.json unless HAVI_CURSOR_CONFIG overrides it.
func ConfigPath() (string, error) {
	if p := os.Getenv("HAVI_CURSOR_CONFIG"); p != "" {
		return p, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home: %w", err)
	}
	return filepath.Join(home, ".cursor", "mcp.json"), nil
}

// Install adds or updates mcpServers.havi in path. Returns
// (StatusConfigured, nil) when the file was written, (StatusAlreadyConfigured,
// nil) when the existing entry already matched, or (StatusFailed, err) on
// error.
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

// Uninstall removes mcpServers.havi from path. Returns (StatusConfigured,
// nil) when something was removed, (StatusAlreadyConfigured, nil) when the
// entry was absent (no-op), or (StatusFailed, err) on error.
//
// When removing havi makes the file represent "nothing havi added"
// (no other top-level keys AND mcpServers is otherwise empty), the file is
// deleted so a fresh install + uninstall round-trip restores the
// pre-install missing-file state.
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

// readOrSeed reads path; if missing or just `{}`, returns the seed value `{}`
// and wasFresh=true. wasFresh signals that the caller may pretty-format the
// result without disturbing user-owned formatting.
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

// jsonEqual reports whether two JSON values are semantically equal regardless
// of whitespace and key ordering.
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
// than mcpServers, and mcpServers itself has no remaining entries.
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
	servers, ok := doc["mcpServers"]
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
