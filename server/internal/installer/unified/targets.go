package unified

import (
	"github.com/handgemacht-ai/annotation-plugin/server/internal/installer/agentsmd"
	"github.com/handgemacht-ai/annotation-plugin/server/internal/installer/codex"
	"github.com/handgemacht-ai/annotation-plugin/server/internal/installer/copilot"
	"github.com/handgemacht-ai/annotation-plugin/server/internal/installer/cursor"
)

// A target is one IDE (or cross-IDE prompts surface) the unified installer
// can write to. The fields are populated once at startup so the orchestrator
// can build the prompt, run the writers, and verify them without re-resolving
// paths.
type target struct {
	key        string
	label      string
	configPath string
	resolveErr error
	detected   bool
	install    func(path string) (string, error)
	uninstall  func(path string) (string, error)
	verify     func(path string) error
}

// allTargets returns the canonical list of supported targets in display order.
// Each target is fully self-describing: detection, path resolution, install,
// uninstall, and post-write verification.
func allTargets() []target {
	out := make([]target, 0, 4)

	out = append(out, codexTarget())
	out = append(out, cursorTarget())
	out = append(out, copilotTarget())
	out = append(out, agentsmdTarget())

	return out
}

func codexTarget() target {
	t := target{
		key:      "codex",
		label:    "Codex CLI (~/.codex/config.toml)",
		detected: codex.DetectCLI(),
		install: func(p string) (string, error) {
			s, err := codex.Install(p)
			return s.String(), err
		},
		uninstall: func(p string) (string, error) {
			s, err := codex.Uninstall(p)
			return s.String(), err
		},
		verify: verifyCodexBlock,
	}
	t.configPath, t.resolveErr = codex.ConfigPath()
	return t
}

func cursorTarget() target {
	t := target{
		key:      "cursor",
		label:    "Cursor (~/.cursor/mcp.json)",
		detected: cursor.DetectCLI(),
		install: func(p string) (string, error) {
			s, err := cursor.Install(p)
			return s.String(), err
		},
		uninstall: func(p string) (string, error) {
			s, err := cursor.Uninstall(p)
			return s.String(), err
		},
		verify: verifyJSONHaviKey(cursor.HaviPath),
	}
	t.configPath, t.resolveErr = cursor.ConfigPath()
	return t
}

func copilotTarget() target {
	t := target{
		key:      "copilot",
		label:    "GitHub Copilot (./.vscode/mcp.json)",
		detected: copilot.DetectCLI(),
		install: func(p string) (string, error) {
			s, err := copilot.Install(p)
			return s.String(), err
		},
		uninstall: func(p string) (string, error) {
			s, err := copilot.Uninstall(p)
			return s.String(), err
		},
		verify: verifyJSONHaviKey(copilot.HaviPath),
	}
	t.configPath, t.resolveErr = copilot.ProjectPath()
	return t
}

func agentsmdTarget() target {
	t := target{
		key:      "agents-md",
		label:    "AGENTS.md (cross-IDE agent prompts)",
		detected: true,
		install: func(p string) (string, error) {
			s, err := agentsmd.Install(p)
			return s.String(), err
		},
		uninstall: func(p string) (string, error) {
			s, err := agentsmd.Uninstall(p)
			return s.String(), err
		},
		verify: verifyAgentsMDBlock,
	}
	t.configPath, t.resolveErr = agentsmd.ProjectPath()
	return t
}
