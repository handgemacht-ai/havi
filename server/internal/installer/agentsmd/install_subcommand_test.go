package agentsmd_test

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

func runHavi(t *testing.T, bin string, env []string, args ...string) (stdout, stderr string, exitCode int) {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Env = append(env, "PATH=/usr/bin:/bin")
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

func TestAgentsMDSubcommand_InstallProjectLocal(t *testing.T) {
	bin := buildHaviBin(t)
	dir := t.TempDir()
	target := filepath.Join(dir, "AGENTS.md")

	env := []string{"HAVI_AGENTSMD_PROJECT_PATH=" + target}
	stdout, stderr, code := runHavi(t, bin, env, "install", "agents-md")
	if code != 0 {
		t.Fatalf("install exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if !strings.HasPrefix(stdout, "agents-md: configured") {
		t.Errorf("expected configured status; got: %s", stdout)
	}
	if !strings.Contains(stdout, target) {
		t.Errorf("expected stdout to mention target path %q; got: %s", target, stdout)
	}

	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read target: %v", err)
	}
	if !bytes.Contains(got, []byte("havi-managed-block")) {
		t.Errorf("written file missing managed block:\n%s", got)
	}
}

func TestAgentsMDSubcommand_InstallGlobal(t *testing.T) {
	bin := buildHaviBin(t)
	dir := t.TempDir()
	target := filepath.Join(dir, "global-AGENTS.md")

	env := []string{"HAVI_AGENTSMD_GLOBAL_PATH=" + target}
	stdout, stderr, code := runHavi(t, bin, env, "install", "agents-md", "--global")
	if code != 0 {
		t.Fatalf("install exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if !strings.Contains(stdout, target) {
		t.Errorf("expected stdout to mention global target %q; got: %s", target, stdout)
	}

	if _, err := os.Stat(target); err != nil {
		t.Errorf("expected global file written; stat err: %v", err)
	}
}

func TestAgentsMDSubcommand_InstallThenUninstall_RoundTrips(t *testing.T) {
	bin := buildHaviBin(t)
	dir := t.TempDir()
	target := filepath.Join(dir, "AGENTS.md")
	seed := []byte("# Project agents\n\nExisting prose.\n")
	if err := os.WriteFile(target, seed, 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}

	env := []string{"HAVI_AGENTSMD_PROJECT_PATH=" + target}

	stdout, _, code := runHavi(t, bin, env, "install", "agents-md")
	if code != 0 {
		t.Fatalf("install exit=%d stdout=%q", code, stdout)
	}
	after1, _ := os.ReadFile(target)

	stdout, _, code = runHavi(t, bin, env, "install", "agents-md")
	if code != 0 {
		t.Fatalf("second install exit=%d", code)
	}
	if !strings.HasPrefix(stdout, "agents-md: already-configured") {
		t.Errorf("expected already-configured on second install; got: %s", stdout)
	}
	after2, _ := os.ReadFile(target)
	if !bytes.Equal(after1, after2) {
		t.Errorf("second install changed file contents (not idempotent)")
	}

	stdout, _, code = runHavi(t, bin, env, "uninstall", "agents-md")
	if code != 0 {
		t.Fatalf("uninstall exit=%d stdout=%q", code, stdout)
	}

	got, _ := os.ReadFile(target)
	if !bytes.Equal(got, seed) {
		t.Errorf("install+uninstall round-trip not byte-identical.\nbefore:\n%s\nafter:\n%s", seed, got)
	}
}

func TestAgentsMDSubcommand_UnsupportedTarget_ExitsTwo(t *testing.T) {
	bin := buildHaviBin(t)
	_, stderr, code := runHavi(t, bin, nil, "install", "nonesuch")
	if code != 2 {
		t.Errorf("expected exit 2 for unsupported target; got %d (stderr: %s)", code, stderr)
	}
	if !strings.Contains(stderr, "unsupported target") {
		t.Errorf("expected unsupported target stderr; got: %s", stderr)
	}
}
