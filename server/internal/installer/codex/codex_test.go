package codex

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestInstall_OnMissingFile_CreatesWithBlock(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "config.toml")

	status, err := Install(path)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if status != StatusConfigured {
		t.Errorf("status = %v, want StatusConfigured", status)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !bytes.Contains(got, []byte(ManagedBlock)) {
		t.Errorf("written file missing managed block:\n%s", got)
	}
	if !bytes.HasSuffix(got, []byte("\n")) {
		t.Errorf("expected trailing newline; got: %q", got)
	}
}

func TestInstall_OnEmptyFile_AppendsBlockWithoutLeadingBlank(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte{}, 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}

	status, err := Install(path)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if status != StatusConfigured {
		t.Errorf("status = %v, want StatusConfigured", status)
	}

	got, _ := os.ReadFile(path)
	if !bytes.HasPrefix(got, []byte(BeginMarker)) {
		t.Errorf("empty-file install should put marker at start; got:\n%s", got)
	}
}

func TestInstall_OnExistingContent_AppendsAfterBlankLine(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	seed := `[[mcp_servers]]
name = "other"
command = "/usr/bin/other"
`
	if err := os.WriteFile(path, []byte(seed), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}

	status, err := Install(path)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if status != StatusConfigured {
		t.Errorf("status = %v, want StatusConfigured", status)
	}

	got, _ := os.ReadFile(path)
	want := seed + "\n" + ManagedBlock + "\n"
	if string(got) != want {
		t.Errorf("unexpected file contents:\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestInstall_IsIdempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	if _, err := Install(path); err != nil {
		t.Fatalf("first Install: %v", err)
	}
	mtime1, err := mtime(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	first, _ := os.ReadFile(path)

	status, err := Install(path)
	if err != nil {
		t.Fatalf("second Install: %v", err)
	}
	if status != StatusAlreadyConfigured {
		t.Errorf("second status = %v, want StatusAlreadyConfigured", status)
	}
	mtime2, _ := mtime(path)
	if !mtime1.Equal(mtime2) {
		t.Errorf("file was rewritten on idempotent install: mtime moved %v -> %v", mtime1, mtime2)
	}
	second, _ := os.ReadFile(path)
	if !bytes.Equal(first, second) {
		t.Errorf("file content changed on idempotent install")
	}
}

func TestInstall_LeavesUnrelatedEntriesUntouched(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	seed := `# user comment
[[mcp_servers]]
name = "other"
command = "/usr/bin/other"
args = ["--flag"]

# another comment
[settings]
foo = "bar"
`
	if err := os.WriteFile(path, []byte(seed), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}

	if _, err := Install(path); err != nil {
		t.Fatalf("Install: %v", err)
	}

	got, _ := os.ReadFile(path)
	if !bytes.HasPrefix(got, []byte(seed)) {
		t.Errorf("install reformatted existing content. got prefix:\n%s", got[:len(seed)])
	}
}

func TestUninstall_RoundTripIsByteIdentical(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	seed := `[[mcp_servers]]
name = "other"
command = "/usr/bin/other"
`
	if err := os.WriteFile(path, []byte(seed), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	original, _ := os.ReadFile(path)

	if _, err := Install(path); err != nil {
		t.Fatalf("Install: %v", err)
	}
	if _, err := Uninstall(path); err != nil {
		t.Fatalf("Uninstall: %v", err)
	}

	after, _ := os.ReadFile(path)
	if !bytes.Equal(original, after) {
		t.Errorf("install+uninstall round-trip not byte-identical.\nbefore:\n%s\nafter:\n%s", original, after)
	}
}

func TestUninstall_OnEmptyOriginalReturnsEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte{}, 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if _, err := Install(path); err != nil {
		t.Fatalf("Install: %v", err)
	}
	if _, err := Uninstall(path); err != nil {
		t.Fatalf("Uninstall: %v", err)
	}

	got, _ := os.ReadFile(path)
	if len(got) != 0 {
		t.Errorf("expected empty file after install/uninstall on empty seed; got %q", got)
	}
}

func TestUninstall_OnFileMissingIsNoOp(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "missing.toml")

	status, err := Uninstall(path)
	if err != nil {
		t.Fatalf("Uninstall on missing file: %v", err)
	}
	if status != StatusAlreadyConfigured {
		t.Errorf("status = %v, want StatusAlreadyConfigured", status)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("Uninstall should not create the file when it doesn't exist")
	}
}

func TestUninstall_OnFileWithoutBlockIsNoOp(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	seed := []byte(`[[mcp_servers]]
name = "other"
`)
	if err := os.WriteFile(path, seed, 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	mtime1, _ := mtime(path)

	status, err := Uninstall(path)
	if err != nil {
		t.Fatalf("Uninstall: %v", err)
	}
	if status != StatusAlreadyConfigured {
		t.Errorf("status = %v, want StatusAlreadyConfigured", status)
	}
	mtime2, _ := mtime(path)
	if !mtime1.Equal(mtime2) {
		t.Errorf("Uninstall rewrote file with no managed block; mtime moved")
	}
}

func TestInstall_ReplacesDriftedBlockInPlace(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	drifted := BeginMarker + `
[[mcp_servers]]
name = "havi"
command = "wrong-binary"
` + EndMarker + "\n"
	prefix := "[other]\nkey = \"value\"\n\n"
	if err := os.WriteFile(path, []byte(prefix+drifted), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}

	status, err := Install(path)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if status != StatusConfigured {
		t.Errorf("status = %v, want StatusConfigured", status)
	}

	got, _ := os.ReadFile(path)
	if !bytes.HasPrefix(got, []byte(prefix)) {
		t.Errorf("Install drifted-block replace reformatted prefix; got:\n%s", got)
	}
	if !bytes.Contains(got, []byte(`args = ["mcp-bridge"]`)) {
		t.Errorf("replaced block missing canonical args; got:\n%s", got)
	}
	if bytes.Contains(got, []byte("wrong-binary")) {
		t.Errorf("drifted entry not replaced; got:\n%s", got)
	}
}

func mtime(path string) (modTime, error) {
	info, err := os.Stat(path)
	if err != nil {
		return modTime{}, err
	}
	return modTime{t: info.ModTime().UnixNano()}, nil
}

type modTime struct{ t int64 }

func (m modTime) Equal(other modTime) bool { return m.t == other.t }
