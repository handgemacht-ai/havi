// Package codex writes and removes havi's MCP server entry in
// ~/.codex/config.toml. The entry is wrapped in a managed-block delimiter so
// the rest of the file is left byte-identical between runs.
package codex

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	BeginMarker = "# >>> havi-managed-block (do not edit, used by `havi install codex`) >>>"
	EndMarker   = "# <<< havi-managed-block <<<"
)

// ManagedBlock is the canonical TOML stanza havi writes. Any value matching
// this string already on disk is considered up-to-date and triggers a no-op.
const ManagedBlock = BeginMarker + `
[[mcp_servers]]
name = "havi"
command = "havi"
args = ["mcp-bridge"]
` + EndMarker

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

// DetectCLI reports whether the Codex CLI is present on PATH. Per the
// ide-detection-cli-only constraint, the IDE is "present" iff its CLI exits
// zero. We invoke `codex --version` rather than relying on file paths.
func DetectCLI() bool {
	bin, err := exec.LookPath("codex")
	if err != nil {
		return false
	}
	cmd := exec.Command(bin, "--version")
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run() == nil
}

// ConfigPath returns the path to ~/.codex/config.toml unless HAVI_CODEX_CONFIG
// is set (test hook).
func ConfigPath() (string, error) {
	if p := os.Getenv("HAVI_CODEX_CONFIG"); p != "" {
		return p, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home: %w", err)
	}
	return filepath.Join(home, ".codex", "config.toml"), nil
}

// Install writes the havi managed block to path. Returns (StatusConfigured,
// nil) when the file was modified, (StatusAlreadyConfigured, nil) when the
// existing block matched, or (StatusFailed, err) on error.
func Install(path string) (Status, error) {
	current, err := readOrEmpty(path)
	if err != nil {
		return StatusFailed, err
	}

	updated, changed := upsertBlock(current, ManagedBlock)
	if !changed {
		return StatusAlreadyConfigured, nil
	}
	if err := writeAtomic(path, updated); err != nil {
		return StatusFailed, err
	}
	return StatusConfigured, nil
}

// Uninstall removes the havi managed block from path. Returns
// (StatusConfigured, nil) when something was removed, (StatusAlreadyConfigured,
// nil) when the block was absent (no-op), or (StatusFailed, err) on error.
func Uninstall(path string) (Status, error) {
	current, err := readOrEmpty(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return StatusAlreadyConfigured, nil
		}
		return StatusFailed, err
	}

	updated, changed := removeBlock(current)
	if !changed {
		return StatusAlreadyConfigured, nil
	}
	if err := writeAtomic(path, updated); err != nil {
		return StatusFailed, err
	}
	return StatusConfigured, nil
}

func readOrEmpty(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	return data, err
}

// upsertBlock returns the contents with the managed block updated to match
// block, and a flag reporting whether the file changed.
//
// Three cases:
//  1. No existing block — append block (preceded by a single blank line if the
//     file is non-empty and doesn't already end with one).
//  2. Existing block byte-identical to block — no change.
//  3. Existing block differs — replace in-place, leaving everything outside
//     the markers byte-identical.
func upsertBlock(current []byte, block string) ([]byte, bool) {
	start, end, ok := findBlock(current)
	if !ok {
		var buf bytes.Buffer
		buf.Write(current)
		if len(current) > 0 && !bytes.HasSuffix(current, []byte("\n")) {
			buf.WriteByte('\n')
		}
		if len(current) > 0 {
			buf.WriteByte('\n')
		}
		buf.WriteString(block)
		buf.WriteByte('\n')
		return buf.Bytes(), true
	}

	existing := string(current[start:end])
	if existing == block {
		return current, false
	}

	var buf bytes.Buffer
	buf.Write(current[:start])
	buf.WriteString(block)
	buf.Write(current[end:])
	return buf.Bytes(), true
}

// removeBlock removes the managed block plus the single blank line that
// upsertBlock inserts before it on first install. Returns the new contents
// and a flag reporting whether anything was removed.
func removeBlock(current []byte) ([]byte, bool) {
	start, end, ok := findBlock(current)
	if !ok {
		return current, false
	}

	// Eat trailing newline after EndMarker so the file doesn't grow an empty
	// line every uninstall round-trip.
	if end < len(current) && current[end] == '\n' {
		end++
	}
	// Eat the single blank line that upsertBlock inserted before the block on
	// first install. Only do this when the preceding two bytes are "\n\n" —
	// that signals a separator we own, not user content.
	if start >= 2 && current[start-1] == '\n' && current[start-2] == '\n' {
		start--
	}

	var buf bytes.Buffer
	buf.Write(current[:start])
	buf.Write(current[end:])
	return buf.Bytes(), true
}

// findBlock returns the byte offsets of the managed block including markers,
// or ok=false if no complete block is present. The block is exactly
// [start, end) where current[start:end] starts with BeginMarker and ends with
// EndMarker. An orphan begin without end (or vice versa) is treated as absent.
func findBlock(current []byte) (start, end int, ok bool) {
	src := string(current)
	bIdx := strings.Index(src, BeginMarker)
	if bIdx < 0 {
		return 0, 0, false
	}
	eRel := strings.Index(src[bIdx:], EndMarker)
	if eRel < 0 {
		return 0, 0, false
	}
	return bIdx, bIdx + eRel + len(EndMarker), true
}

func writeAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}
	tmp, err := os.CreateTemp(dir, ".codex-config.toml.*")
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
