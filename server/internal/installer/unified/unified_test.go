package unified

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeTarget builds a target with controllable install/uninstall/verify
// closures so we can exercise the orchestrator without touching real IDE
// configs.
func fakeTarget(key string, detected bool, status string, installErr, verifyErr error) target {
	return target{
		key:        key,
		label:      key + " (fake)",
		detected:   detected,
		configPath: "/tmp/fake/" + key,
		install: func(p string) (string, error) {
			if installErr != nil {
				return "failed", installErr
			}
			return status, nil
		},
		uninstall: func(p string) (string, error) {
			return "configured", nil
		},
		verify: func(p string) error { return verifyErr },
	}
}

func TestRunOne_InstallSuccess_AppliesVerify(t *testing.T) {
	tgt := fakeTarget("x", true, "configured", nil, nil)
	r := runOne(tgt, ActionInstall)
	if r.Failed() {
		t.Errorf("expected success, got: %+v", r)
	}
	if r.Status != "configured" {
		t.Errorf("status=%q want configured", r.Status)
	}
}

func TestRunOne_VerifyFailureMarksFailed(t *testing.T) {
	tgt := fakeTarget("x", true, "configured", nil, errors.New("missing key"))
	r := runOne(tgt, ActionInstall)
	if !r.Failed() {
		t.Errorf("expected failure when verifier rejects; got: %+v", r)
	}
	if !strings.Contains(r.Err.Error(), "smoke probe") {
		t.Errorf("expected err to mention smoke probe; got: %v", r.Err)
	}
}

func TestRunOne_InstallErrorIsFinal(t *testing.T) {
	verifierCalled := false
	tgt := target{
		key:        "x",
		configPath: "/tmp/x",
		install: func(p string) (string, error) {
			return "failed", errors.New("disk full")
		},
		verify: func(p string) error { verifierCalled = true; return nil },
	}
	r := runOne(tgt, ActionInstall)
	if !r.Failed() {
		t.Errorf("expected failed result")
	}
	if verifierCalled {
		t.Errorf("verifier should not run after install error")
	}
}

func TestRunOne_UninstallSkipsVerify(t *testing.T) {
	verifierCalled := false
	tgt := target{
		key:        "x",
		configPath: "/tmp/x",
		uninstall: func(p string) (string, error) {
			return "configured", nil
		},
		verify: func(p string) error { verifierCalled = true; return errors.New("would fail") },
	}
	r := runOne(tgt, ActionUninstall)
	if r.Failed() {
		t.Errorf("uninstall should not invoke verifier; got: %+v", r)
	}
	if verifierCalled {
		t.Errorf("verifier ran on uninstall path")
	}
}

func TestChooseTargets_ExplicitIDEs_OverrideDetection(t *testing.T) {
	all := []target{
		fakeTarget("a", false, "configured", nil, nil),
		fakeTarget("b", true, "configured", nil, nil),
	}
	keyed := indexTargets(all)
	picked, err := chooseTargets(Options{IDEs: []string{"a"}}, all, keyed, os.Stderr)
	if err != nil {
		t.Fatalf("choose: %v", err)
	}
	if len(picked) != 1 || picked[0].key != "a" {
		t.Errorf("expected only a; got: %+v", picked)
	}
}

func TestChooseTargets_UnknownIDE_IsError(t *testing.T) {
	all := []target{fakeTarget("a", true, "configured", nil, nil)}
	keyed := indexTargets(all)
	if _, err := chooseTargets(Options{IDEs: []string{"nonesuch"}}, all, keyed, os.Stderr); err == nil {
		t.Errorf("expected error on unknown target")
	}
}

func TestChooseTargets_All_SelectsEveryTarget(t *testing.T) {
	all := []target{
		fakeTarget("a", false, "configured", nil, nil),
		fakeTarget("b", false, "configured", nil, nil),
	}
	keyed := indexTargets(all)
	picked, err := chooseTargets(Options{All: true}, all, keyed, os.Stderr)
	if err != nil {
		t.Fatalf("choose: %v", err)
	}
	if len(picked) != 2 {
		t.Errorf("expected all 2; got: %+v", picked)
	}
}

func TestChooseTargets_Yes_OnlySelectsDetected(t *testing.T) {
	all := []target{
		fakeTarget("a", false, "configured", nil, nil),
		fakeTarget("b", true, "configured", nil, nil),
	}
	keyed := indexTargets(all)
	picked, err := chooseTargets(Options{Yes: true}, all, keyed, os.Stderr)
	if err != nil {
		t.Fatalf("choose: %v", err)
	}
	if len(picked) != 1 || picked[0].key != "b" {
		t.Errorf("expected only detected (b); got: %+v", picked)
	}
}

func TestPrintSummary_RendersOneRowPerResult(t *testing.T) {
	var buf bytes.Buffer
	printSummary(&buf, ActionInstall, []Result{
		{Key: "codex", Status: "configured", Path: "/x/config.toml"},
		{Key: "cursor", Status: "failed", Path: "/x/mcp.json", Err: errors.New("permission denied")},
	})
	out := buf.String()
	if !strings.Contains(out, "✓") || !strings.Contains(out, "codex") || !strings.Contains(out, "configured") {
		t.Errorf("missing success row: %s", out)
	}
	if !strings.Contains(out, "✗") || !strings.Contains(out, "cursor") || !strings.Contains(out, "permission denied") {
		t.Errorf("missing failure row: %s", out)
	}
}

func TestVerifyJSONHaviKey_ChecksCommandAndArgs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mcp.json")
	if err := os.WriteFile(path, []byte(`{"mcpServers":{"havi":{"command":"havi","args":["mcp-bridge"]}}}`), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	verify := verifyJSONHaviKey("mcpServers.havi")
	if err := verify(path); err != nil {
		t.Errorf("expected pass; got: %v", err)
	}
}

func TestVerifyJSONHaviKey_FailsOnDriftedCommand(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mcp.json")
	if err := os.WriteFile(path, []byte(`{"mcpServers":{"havi":{"command":"wrong","args":["mcp-bridge"]}}}`), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	verify := verifyJSONHaviKey("mcpServers.havi")
	if err := verify(path); err == nil {
		t.Errorf("expected failure on drifted command")
	}
}

func TestVerifyJSONHaviKey_FailsOnMissingFile(t *testing.T) {
	verify := verifyJSONHaviKey("mcpServers.havi")
	if err := verify("/nonexistent/path.json"); err == nil {
		t.Errorf("expected failure on missing file")
	}
}

func TestServerHealth_Unreachable(t *testing.T) {
	_, ok := serverHealth(context.Background(), "1")
	if ok {
		t.Errorf("expected unreachable on port 1")
	}
}

func TestRun_PartialFailure_IsolatesAndExitsNonZero(t *testing.T) {
	dir := t.TempDir()

	t.Setenv("HAVI_CODEX_CONFIG", filepath.Join(dir, "codex", "config.toml"))
	t.Setenv("HAVI_CURSOR_CONFIG", filepath.Join(dir, "cursor", "mcp.json"))
	t.Setenv("HAVI_COPILOT_PROJECT_PATH", filepath.Join(dir, "copilot", "mcp.json"))

	unwritable := filepath.Join(dir, "blocker")
	if err := os.WriteFile(unwritable, []byte("not a dir"), 0o644); err != nil {
		t.Fatalf("seed blocker: %v", err)
	}
	t.Setenv("HAVI_AGENTSMD_PROJECT_PATH", filepath.Join(unwritable, "AGENTS.md"))

	var out, errBuf bytes.Buffer
	results, err := Run(context.Background(), Options{
		IDEs:   []string{"cursor", "agents-md"},
		Stdout: &out,
		Stderr: &errBuf,
	})
	if err == nil {
		t.Errorf("expected non-nil error when one target fails")
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got: %d", len(results))
	}
	var cursorRes, agentsRes *Result
	for i := range results {
		switch results[i].Key {
		case "cursor":
			cursorRes = &results[i]
		case "agents-md":
			agentsRes = &results[i]
		}
	}
	if cursorRes == nil || cursorRes.Failed() {
		t.Errorf("cursor should have succeeded; got: %+v", cursorRes)
	}
	if agentsRes == nil || !agentsRes.Failed() {
		t.Errorf("agents-md should have failed (parent path is a file); got: %+v", agentsRes)
	}
}
