package copilot

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tidwall/gjson"
)

func TestInstall_OnMissingFile_CreatesPrettyFormatted(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "mcp.json")

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
	cmd := gjson.GetBytes(got, "servers.havi.command").String()
	if cmd != "havi" {
		t.Errorf("servers.havi.command = %q, want %q", cmd, "havi")
	}
	args := gjson.GetBytes(got, "servers.havi.args").Array()
	if len(args) != 1 || args[0].String() != "mcp-bridge" {
		t.Errorf("servers.havi.args = %v, want [\"mcp-bridge\"]", args)
	}
	if !bytes.Contains(got, []byte("\n  ")) {
		t.Errorf("expected pretty-formatted output (2-space indent); got:\n%s", got)
	}
}

func TestInstall_OnFileWithOtherServers_PreservesThem(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mcp.json")
	seed := []byte(`{
  "servers": {
    "github": {
      "type": "http",
      "url": "https://api.githubcopilot.com/mcp"
    },
    "playwright": {
      "command": "npx",
      "args": ["-y", "@microsoft/mcp-server-playwright"]
    }
  }
}
`)
	if err := os.WriteFile(path, seed, 0o644); err != nil {
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
	if gjson.GetBytes(got, "servers.github.url").String() != "https://api.githubcopilot.com/mcp" {
		t.Errorf("unrelated servers.github entry lost; got:\n%s", got)
	}
	if gjson.GetBytes(got, "servers.playwright.command").String() != "npx" {
		t.Errorf("unrelated servers.playwright entry lost; got:\n%s", got)
	}
	if !gjson.GetBytes(got, "servers.havi").Exists() {
		t.Errorf("havi entry not added:\n%s", got)
	}
}

func TestInstall_OnFileWithOtherTopLevelKeys_PreservesThem(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mcp.json")
	seed := []byte(`{"inputs":[{"id":"x"}],"servers":{}}`)
	if err := os.WriteFile(path, seed, 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}

	if _, err := Install(path); err != nil {
		t.Fatalf("Install: %v", err)
	}

	got, _ := os.ReadFile(path)
	if gjson.GetBytes(got, "inputs.0.id").String() != "x" {
		t.Errorf("unrelated top-level key lost; got:\n%s", got)
	}
}

func TestInstall_IsIdempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mcp.json")

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

func TestInstall_ReplacesDriftedHaviEntry(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mcp.json")
	seed := []byte(`{"servers":{"havi":{"command":"wrong-binary","args":["wrong"]}}}`)
	if err := os.WriteFile(path, seed, 0o644); err != nil {
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
	if gjson.GetBytes(got, "servers.havi.command").String() != "havi" {
		t.Errorf("drifted command not replaced; got:\n%s", got)
	}
	if gjson.GetBytes(got, "servers.havi.args.0").String() != "mcp-bridge" {
		t.Errorf("drifted args not replaced; got:\n%s", got)
	}
}

func TestUninstall_OnMissingFile_IsNoOp(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "missing.json")

	status, err := Uninstall(path)
	if err != nil {
		t.Fatalf("Uninstall: %v", err)
	}
	if status != StatusAlreadyConfigured {
		t.Errorf("status = %v, want StatusAlreadyConfigured", status)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("Uninstall created file when missing")
	}
}

func TestUninstall_OnFileWithoutHavi_IsNoOp(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mcp.json")
	seed := []byte(`{"servers":{"other":{"command":"other-cli"}}}`)
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
		t.Errorf("Uninstall touched file with no havi entry; mtime moved")
	}
}

func TestInstallUninstall_RestoresMissingFileWhenWeWereOnlyContent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mcp.json")

	if _, err := Install(path); err != nil {
		t.Fatalf("Install: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("file should exist after install: %v", err)
	}

	if _, err := Uninstall(path); err != nil {
		t.Fatalf("Uninstall: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("file should be deleted on uninstall when havi was sole content")
	}
}

func TestInstallUninstall_PreservesOtherServersSemantically(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mcp.json")
	seed := []byte(`{
  "servers": {
    "github": {
      "type": "http",
      "url": "https://api.githubcopilot.com/mcp"
    }
  }
}
`)
	if err := os.WriteFile(path, seed, 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}

	if _, err := Install(path); err != nil {
		t.Fatalf("Install: %v", err)
	}
	if _, err := Uninstall(path); err != nil {
		t.Fatalf("Uninstall: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("file should still exist (other entries remain): %v", err)
	}
	if !jsonStructuralEqual(t, got, seed) {
		t.Errorf("install+uninstall not structurally equivalent.\nbefore:\n%s\nafter:\n%s", seed, got)
	}
}

func TestProjectPath_HonorsEnvOverride(t *testing.T) {
	t.Setenv("HAVI_COPILOT_PROJECT_PATH", "/tmp/proj/.vscode/mcp.json")
	got, err := ProjectPath()
	if err != nil {
		t.Fatalf("ProjectPath: %v", err)
	}
	if got != "/tmp/proj/.vscode/mcp.json" {
		t.Errorf("ProjectPath = %q, want override", got)
	}
}

func TestGlobalPath_HonorsEnvOverride(t *testing.T) {
	t.Setenv("HAVI_COPILOT_GLOBAL_PATH", "/tmp/user/mcp.json")
	got, err := GlobalPath()
	if err != nil {
		t.Fatalf("GlobalPath: %v", err)
	}
	if got != "/tmp/user/mcp.json" {
		t.Errorf("GlobalPath = %q, want override", got)
	}
}

func TestInstall_OnMalformedJSON_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mcp.json")
	if err := os.WriteFile(path, []byte("{not valid json"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}

	_, err := Install(path)
	if err == nil {
		t.Errorf("Install should fail on malformed JSON; got nil")
	}
	if err != nil && !strings.Contains(err.Error(), "parse") {
		t.Logf("error (may be acceptable): %v", err)
	}
}

func jsonStructuralEqual(t *testing.T, a, b []byte) bool {
	t.Helper()
	var av, bv any
	if err := json.Unmarshal(a, &av); err != nil {
		t.Fatalf("unmarshal a: %v", err)
	}
	if err := json.Unmarshal(b, &bv); err != nil {
		t.Fatalf("unmarshal b: %v", err)
	}
	aOut, _ := json.Marshal(av)
	bOut, _ := json.Marshal(bv)
	return bytes.Equal(aOut, bOut)
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
