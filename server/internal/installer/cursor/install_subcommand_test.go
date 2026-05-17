package cursor_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func buildHaviBin(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "havi")
	out, err := exec.Command("go", "build", "-o", bin, "../../../cmd/server").CombinedOutput()
	if err != nil {
		t.Fatalf("build havi binary: %v\n%s", err, out)
	}
	return bin
}

func runHavi(t *testing.T, bin, configPath string, args ...string) (stdout, stderr string, exitCode int) {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Env = []string{
		"HAVI_CURSOR_CONFIG=" + configPath,
		"HOME=" + filepath.Dir(configPath),
		"PATH=/usr/bin:/bin",
	}
	var sout, serr bytes.Buffer
	cmd.Stdout = &sout
	cmd.Stderr = &serr
	err := cmd.Run()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			exitCode = ee.ExitCode()
		} else {
			t.Fatalf("run %v: %v", args, err)
		}
	}
	return sout.String(), serr.String(), exitCode
}

func TestSubcommand_Cursor_InstallThenUninstall_RoundTrips(t *testing.T) {
	bin := buildHaviBin(t)
	dir := t.TempDir()
	configPath := filepath.Join(dir, "mcp.json")
	seed := []byte(`{
  "mcpServers": {
    "other": {
      "command": "other-cli",
      "args": ["--flag"]
    }
  }
}
`)
	if err := os.WriteFile(configPath, seed, 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}

	stdout, stderr, code := runHavi(t, bin, configPath, "install", "cursor")
	if code != 0 {
		t.Fatalf("first install exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if !strings.HasPrefix(stdout, "cursor: configured") {
		t.Errorf("expected configured status; got: %s", stdout)
	}
	after1, _ := os.ReadFile(configPath)

	stdout, _, code = runHavi(t, bin, configPath, "install", "cursor")
	if code != 0 {
		t.Fatalf("second install exit=%d", code)
	}
	if !strings.HasPrefix(stdout, "cursor: already-configured") {
		t.Errorf("expected already-configured status; got: %s", stdout)
	}
	after2, _ := os.ReadFile(configPath)
	if !bytes.Equal(after1, after2) {
		t.Errorf("second install changed file contents (not idempotent)")
	}

	stdout, _, code = runHavi(t, bin, configPath, "uninstall", "cursor")
	if code != 0 {
		t.Fatalf("uninstall exit=%d", code)
	}
	if !strings.HasPrefix(stdout, "cursor: configured") {
		t.Errorf("expected uninstall to report configured; got: %s", stdout)
	}

	got, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("file should still exist with other entry: %v", err)
	}
	if !bytes.Contains(got, []byte("other-cli")) {
		t.Errorf("unrelated entry lost; got:\n%s", got)
	}
	if bytes.Contains(got, []byte("\"havi\"")) {
		t.Errorf("havi entry not removed; got:\n%s", got)
	}
}

func TestSubcommand_Cursor_UninstallOnMissingFile_IsNoOp(t *testing.T) {
	bin := buildHaviBin(t)
	dir := t.TempDir()
	configPath := filepath.Join(dir, "missing.json")

	stdout, _, code := runHavi(t, bin, configPath, "uninstall", "cursor")
	if code != 0 {
		t.Fatalf("uninstall exit=%d stdout=%q", code, stdout)
	}
	if !strings.HasPrefix(stdout, "cursor: already-configured") {
		t.Errorf("expected already-configured for missing file; got: %s", stdout)
	}
	if _, err := os.Stat(configPath); !os.IsNotExist(err) {
		t.Errorf("uninstall created file when missing")
	}
}
