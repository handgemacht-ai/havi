// Package agentsmd writes and removes havi's annotation-handling instructions
// in an AGENTS.md file (Linux Foundation standard, December 2025; read by
// Codex, Cursor, GitHub Copilot, Gemini CLI, Windsurf, and others). The block
// is wrapped in HTML-comment delimiters so the rest of the file is left
// byte-identical between runs.
package agentsmd

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	BeginMarker = "<!-- >>> havi-managed-block (do not edit, used by `havi install agents-md`) >>> -->"
	EndMarker   = "<!-- <<< havi-managed-block <<< -->"
)

// ManagedBlock is the canonical Markdown stanza havi writes. Any value matching
// this string already on disk is considered up-to-date and triggers a no-op.
const ManagedBlock = BeginMarker + `
## havi annotations

havi is a browser-side annotation tool. Developers capture visual and technical observations from a web page; annotations land in this project's havi MCP server and surface to coding agents through the ` + "`mcp__annotation__*`" + ` tool family.

### Available tools

- ` + "`mcp__annotation__list_annotations`" + ` — list and filter annotations; the response contains the full W3C envelope for each match
- ` + "`mcp__annotation__get_annotation_image`" + ` — get the screenshot for one annotation as a base64 image
- ` + "`mcp__annotation__resolve_annotation`" + ` — mark an annotation resolved with metadata

### Git context preamble

Before any havi tool call that takes branch, commit, or worktree parameters, gather git context:

- Branch: ` + "`git rev-parse --abbrev-ref HEAD`" + `
- Commit: ` + "`git rev-parse HEAD`" + `
- Worktree: ` + "`git rev-parse --show-toplevel`" + `

Pass the branch as the ` + "`branch`" + ` parameter when calling ` + "`mcp__annotation__list_annotations`" + `, and pass the commit in the ` + "`metadata.commit`" + ` field when calling ` + "`mcp__annotation__resolve_annotation`" + `.

### Reviewing open annotations

When the user asks to review annotations, fix visual bugs, or process annotation feedback:

1. Call ` + "`mcp__annotation__list_annotations`" + ` with ` + "`state: open`" + ` and the ` + "`branch`" + ` from the preamble above
2. If the response contains no annotations, tell the user and stop
3. For each annotation in the response, in creation order (oldest first), follow "Resolving a single annotation" below

### Resolving a single annotation

For each annotation returned by ` + "`list_annotations`" + `:

1. Read the annotation's ` + "`body`" + `, ` + "`target.selector`" + ` (` + "`CssSelector`" + ` / ` + "`FragmentSelector`" + `), ` + "`target.source`" + ` (the page URL), and any ` + "`describing`" + ` body entries (console errors, network failures, web vitals) — all already in the list response
2. Call ` + "`mcp__annotation__get_annotation_image`" + ` with the annotation ` + "`id`" + ` to view the screenshot
3. Locate source code: use the target URL path to identify the affected route or page; use the CSS selector with Grep to find the component or template that renders the annotated element
4. Make a minimal fix — address exactly what the annotation describes; do not refactor, add features, or clean up surrounding code
5. Call ` + "`mcp__annotation__resolve_annotation`" + ` with the annotation ` + "`id`" + ` and ` + "`metadata: { commit: <new HEAD after fix>, note: <one-line fix description> }`" + `

If the source cannot be located or the fix is unclear, report what you found and ask the user rather than guessing.
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

// ProjectPath returns the project-local AGENTS.md path: AGENTS.md in cwd.
// Honors HAVI_AGENTSMD_PROJECT_PATH for tests.
func ProjectPath() (string, error) {
	if p := os.Getenv("HAVI_AGENTSMD_PROJECT_PATH"); p != "" {
		return p, nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("resolve cwd: %w", err)
	}
	return filepath.Join(cwd, "AGENTS.md"), nil
}

// GlobalPath returns the user-global path: ~/.codex/AGENTS.md.
// Honors HAVI_AGENTSMD_GLOBAL_PATH for tests.
func GlobalPath() (string, error) {
	if p := os.Getenv("HAVI_AGENTSMD_GLOBAL_PATH"); p != "" {
		return p, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home: %w", err)
	}
	return filepath.Join(home, ".codex", "AGENTS.md"), nil
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

	if end < len(current) && current[end] == '\n' {
		end++
	}
	if start >= 2 && current[start-1] == '\n' && current[start-2] == '\n' {
		start--
	}

	var buf bytes.Buffer
	buf.Write(current[:start])
	buf.Write(current[end:])
	return buf.Bytes(), true
}

// findBlock returns the byte offsets of the managed block including markers,
// or ok=false if no complete block is present. An orphan begin without end
// (or vice versa) is treated as absent.
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
	tmp, err := os.CreateTemp(dir, ".AGENTS.md.*")
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
