package codex_test

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

// fakeCodexDir writes a stub `codex` binary on a fresh PATH directory and
// returns the directory.
func fakeCodexDir(t *testing.T, exitCode int) string {
	t.Helper()
	dir := t.TempDir()
	script := "#!/usr/bin/env bash\nexit " + itoa(exitCode) + "\n"
	if err := os.WriteFile(filepath.Join(dir, "codex"), []byte(script), 0o755); err != nil {
		t.Fatalf("write stub: %v", err)
	}
	return dir
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	negative := n < 0
	if negative {
		n = -n
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	if negative {
		b = append([]byte{'-'}, b...)
	}
	return string(b)
}

func runHavi(t *testing.T, bin, configPath string, codexBinDir string, args ...string) (stdout, stderr string, exitCode int) {
	t.Helper()
	cmd := exec.Command(bin, args...)
	env := []string{
		"HAVI_CODEX_CONFIG=" + configPath,
		"HOME=" + filepath.Dir(configPath),
	}
	basePath := "/usr/bin:/bin"
	if codexBinDir != "" {
		env = append(env, "PATH="+codexBinDir+":"+basePath)
	} else {
		env = append(env, "PATH="+basePath)
	}
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

func TestSubcommand_Install_RequiresCodexCLI(t *testing.T) {
	bin := buildHaviBin(t)
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")

	_, stderr, code := runHavi(t, bin, configPath, "", "install", "codex")
	if code == 0 {
		t.Errorf("expected non-zero exit when codex CLI absent; got 0")
	}
	if !strings.Contains(stderr, "Codex CLI not on PATH") {
		t.Errorf("expected install-hint stderr; got: %s", stderr)
	}
	if _, err := os.Stat(configPath); !os.IsNotExist(err) {
		t.Errorf("install must not write config when CLI absent (stat err: %v)", err)
	}
}

func TestSubcommand_InstallThenUninstall_IsIdempotentAndRoundTrips(t *testing.T) {
	bin := buildHaviBin(t)
	codexBin := fakeCodexDir(t, 0)
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")
	seed := []byte("[[mcp_servers]]\nname = \"other\"\ncommand = \"/bin/other\"\n")
	if err := os.WriteFile(configPath, seed, 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}

	stdout, stderr, code := runHavi(t, bin, configPath, codexBin, "install", "codex")
	if code != 0 {
		t.Fatalf("first install exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if !strings.HasPrefix(stdout, "codex: configured") {
		t.Errorf("expected configured status; got: %s", stdout)
	}
	after1, _ := os.ReadFile(configPath)

	stdout, _, code = runHavi(t, bin, configPath, codexBin, "install", "codex")
	if code != 0 {
		t.Fatalf("second install exit=%d", code)
	}
	if !strings.HasPrefix(stdout, "codex: already-configured") {
		t.Errorf("expected already-configured status; got: %s", stdout)
	}
	after2, _ := os.ReadFile(configPath)
	if !bytes.Equal(after1, after2) {
		t.Errorf("second install changed file contents (not idempotent)")
	}

	stdout, _, code = runHavi(t, bin, configPath, codexBin, "uninstall", "codex")
	if code != 0 {
		t.Fatalf("uninstall exit=%d", code)
	}
	if !strings.HasPrefix(stdout, "codex: configured") {
		t.Errorf("expected uninstall to report configured (file written); got: %s", stdout)
	}

	got, _ := os.ReadFile(configPath)
	if !bytes.Equal(got, seed) {
		t.Errorf("install+uninstall round-trip not byte-identical.\nbefore:\n%s\nafter:\n%s", seed, got)
	}
}

func TestSubcommand_UnsupportedIDE_ExitsTwo(t *testing.T) {
	bin := buildHaviBin(t)
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")

	_, stderr, code := runHavi(t, bin, configPath, "", "install", "nonesuch")
	if code != 2 {
		t.Errorf("expected exit 2 for unsupported IDE; got %d (stderr: %s)", code, stderr)
	}
	if !strings.Contains(stderr, "unsupported IDE") {
		t.Errorf("expected unsupported IDE stderr; got: %s", stderr)
	}
}
