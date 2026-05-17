package unified_test

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

// stubCodexDir writes a fake `codex` binary on PATH so codex.Install's
// upstream `codex --version` precondition (used by the per-target wrapper, not
// the unified path — but harmless to include for parity) is satisfied.
func stubCodexDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "codex"), []byte("#!/usr/bin/env bash\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write stub: %v", err)
	}
	return dir
}

func runHavi(t *testing.T, bin string, env []string, args ...string) (stdout, stderr string, exitCode int) {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Env = env
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

func TestUnified_Install_NonInteractive_AllFourTargets(t *testing.T) {
	bin := buildHaviBin(t)
	dir := t.TempDir()
	codexPath := filepath.Join(dir, "codex", "config.toml")
	cursorPath := filepath.Join(dir, "cursor", "mcp.json")
	copilotPath := filepath.Join(dir, "copilot", "mcp.json")
	agentsPath := filepath.Join(dir, "AGENTS.md")
	codexBin := stubCodexDir(t)

	env := []string{
		"PATH=" + codexBin + ":/usr/bin:/bin",
		"HOME=" + dir,
		"HAVI_CODEX_CONFIG=" + codexPath,
		"HAVI_CURSOR_CONFIG=" + cursorPath,
		"HAVI_COPILOT_PROJECT_PATH=" + copilotPath,
		"HAVI_AGENTSMD_PROJECT_PATH=" + agentsPath,
	}

	stdout, stderr, code := runHavi(t, bin, env, "install", "--ides=codex,cursor,copilot,agents-md", "--port=1")
	if code != 0 {
		t.Fatalf("unified install exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "summary") {
		t.Errorf("expected summary section; got: %s", stdout)
	}
	for _, key := range []string{"codex", "cursor", "copilot", "agents-md"} {
		if !strings.Contains(stdout, key) {
			t.Errorf("summary missing %s: %s", key, stdout)
		}
	}
	if strings.Count(stdout, "✓") < 4 {
		t.Errorf("expected at least 4 success rows; got: %s", stdout)
	}
	for _, p := range []string{codexPath, cursorPath, copilotPath, agentsPath} {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("expected file %s; stat err: %v", p, err)
		}
	}

	stdout, _, code = runHavi(t, bin, env, "install", "--ides=codex,cursor,copilot,agents-md", "--port=1")
	if code != 0 {
		t.Fatalf("second install exit=%d", code)
	}
	if strings.Count(stdout, "already-configured") < 4 {
		t.Errorf("expected idempotent second run; got: %s", stdout)
	}
}

func TestUnified_Install_PartialFailure_IsolatesAndExitsNonZero(t *testing.T) {
	bin := buildHaviBin(t)
	dir := t.TempDir()
	cursorPath := filepath.Join(dir, "cursor", "mcp.json")

	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("not a dir"), 0o644); err != nil {
		t.Fatalf("seed blocker: %v", err)
	}
	agentsBlocked := filepath.Join(blocker, "AGENTS.md")

	env := []string{
		"PATH=/usr/bin:/bin",
		"HOME=" + dir,
		"HAVI_CURSOR_CONFIG=" + cursorPath,
		"HAVI_AGENTSMD_PROJECT_PATH=" + agentsBlocked,
	}

	stdout, _, code := runHavi(t, bin, env, "install", "--ides=cursor,agents-md", "--port=1")
	if code == 0 {
		t.Errorf("expected non-zero exit when one target fails; got 0")
	}
	if !strings.Contains(stdout, "✓") {
		t.Errorf("expected cursor to succeed; got: %s", stdout)
	}
	if !strings.Contains(stdout, "✗") {
		t.Errorf("expected agents-md to fail with ✗; got: %s", stdout)
	}
	if _, err := os.Stat(cursorPath); err != nil {
		t.Errorf("cursor file should be written despite agents-md failure; got: %v", err)
	}
}

func TestUnified_Install_UnknownIDE_Errors(t *testing.T) {
	bin := buildHaviBin(t)
	dir := t.TempDir()
	env := []string{"PATH=/usr/bin:/bin", "HOME=" + dir}

	_, _, code := runHavi(t, bin, env, "install", "--ides=nonesuch", "--port=1")
	if code == 0 {
		t.Errorf("expected non-zero on unknown target")
	}
}

func TestUnified_Uninstall_RoundTrips(t *testing.T) {
	bin := buildHaviBin(t)
	dir := t.TempDir()
	cursorPath := filepath.Join(dir, "cursor", "mcp.json")

	env := []string{
		"PATH=/usr/bin:/bin",
		"HOME=" + dir,
		"HAVI_CURSOR_CONFIG=" + cursorPath,
	}

	if _, _, code := runHavi(t, bin, env, "install", "--ides=cursor", "--port=1"); code != 0 {
		t.Fatalf("install exit=%d", code)
	}
	if _, err := os.Stat(cursorPath); err != nil {
		t.Fatalf("install should create file: %v", err)
	}

	if _, _, code := runHavi(t, bin, env, "uninstall", "--ides=cursor", "--port=1"); code != 0 {
		t.Fatalf("uninstall exit=%d", code)
	}
	if _, err := os.Stat(cursorPath); !os.IsNotExist(err) {
		t.Errorf("uninstall should delete file when havi was sole content; got: %v", err)
	}
}

func TestUnified_BackwardCompat_PerIDESubcommandStillWorks(t *testing.T) {
	bin := buildHaviBin(t)
	dir := t.TempDir()
	cursorPath := filepath.Join(dir, "cursor", "mcp.json")

	env := []string{
		"PATH=/usr/bin:/bin",
		"HOME=" + dir,
		"HAVI_CURSOR_CONFIG=" + cursorPath,
	}

	stdout, _, code := runHavi(t, bin, env, "install", "cursor")
	if code != 0 {
		t.Fatalf("per-IDE install exit=%d stdout=%q", code, stdout)
	}
	if !strings.HasPrefix(stdout, "cursor: configured") {
		t.Errorf("per-IDE install should still produce single-line output; got: %s", stdout)
	}
}
