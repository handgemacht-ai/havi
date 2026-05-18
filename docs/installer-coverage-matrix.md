# Installer Coverage Matrix

Per epic au-uamx Outcome 8. Lists every {IDE × OS × scenario} cell exercised by `havi install` and `havi uninstall` with its current verification status.

**OS scope.** The epic requires "at least one green CI job per IDE on Linux"; macOS / Windows are out of scope for auto-CI in this iteration and intentionally listed as `manual-checklist` for the cells where OS-specific behavior is plausible (path resolution, file permissions). Every cell below is `ubuntu-latest` unless otherwise marked.

**Status values:**

- `auto-CI` — exercised by `.github/workflows/installer-ci.yml`. A red status on the relevant job blocks merge.
- `manual-checklist` — verifiable but not automated. Carries a one-line justification.
- `N/A` — does not apply to this IDE / OS combination.

## Matrix (Linux)

| Scenario | Claude | Cursor | Copilot | Codex |
|---|---|---|---|---|
| Outcome 1 — multi-IDE installer reaches every detected IDE | auto-CI | auto-CI | auto-CI | auto-CI |
| Outcome 2 — browser annotation visible via IDE's named list-annotations affordance | manual-checklist | manual-checklist | manual-checklist | manual-checklist |
| Outcome 3 — annotation resolvable via IDE's named resolver affordance | manual-checklist | manual-checklist | manual-checklist | manual-checklist |
| Outcome 4 (happy) — auto-revival of dead local server | auto-CI | manual-checklist | manual-checklist | auto-CI |
| Outcome 4 (opt-out) — `HAVI_NO_AUTO_REVIVE` honored | auto-CI | manual-checklist | manual-checklist | auto-CI |
| Outcome 5 — re-running installer is a no-op (idempotency) | auto-CI | auto-CI | auto-CI | auto-CI |
| Outcome 6 — partial install isolates failure | auto-CI | auto-CI | auto-CI | auto-CI |
| Outcome 7 — uninstall leaves zero residue | auto-CI | auto-CI | auto-CI | auto-CI |

Cells per IDE: 8 (Outcomes 1–7, with Outcome 4 split happy/opt-out). Total cells = 32. Auto-CI = 20. Manual-checklist = 12. N/A = 0.

## Auto-CI coverage by job

The four CI jobs in `.github/workflows/installer-ci.yml` exercise the cells as follows:

- `codex-linux` — `server/internal/installer/codex/...` (Outcomes 5, 6, 7 for Codex via `codex_test.go`, `install_subcommand_test.go`) + `server/internal/installer/unified/...` (Outcomes 1, 5, 6 for all four IDEs via `unified_test.go`, `install_subcommand_test.go`).
- `cursor-linux` — `server/internal/installer/cursor/...` + `server/internal/installer/unified/...`.
- `copilot-linux` — `server/internal/installer/copilot/...` + `server/internal/installer/unified/...`.
- `claude-linux` — `server/internal/installer/agentsmd/...` + `server/internal/installer/unified/...` + `plugin/tests/ensure-server-test.sh` (covers Outcome 4 happy & opt-out for the Claude SessionStart hook).

The unified package's `TestUnified_Install_NonInteractive_AllFourTargets` exercises Outcome 1 across every column in every job. `TestUnified_Install_PartialFailure_IsolatesAndExitsNonZero` exercises Outcome 6. The per-IDE writer tests (`TestInstall_IsIdempotent`, `TestInstallUninstall_PreservesOtherServersSemantically`) exercise Outcomes 5 and 7. The Codex job exercises Outcome 4 through `havi mcp-bridge` via `plugin/tests/ensure-server-test.sh` cases `auto-revive-default-on` and `opt-out-honored`.

## Justifications for non-auto-CI cells

- **Outcome 2 — all IDEs — `manual-checklist`**: The scenario asserts that each IDE's _own named havi affordance_ (slash command in Claude / Cursor, chat mode in Copilot, subagent in Codex) returns the captured annotation. Verifying this end-to-end requires a live IDE process driving the MCP transport; CI cannot host Cursor/Copilot/Codex/Claude UI sessions. The MCP tool surface itself IS auto-CI'd via `server/internal/mcp` tests and via the per-IDE writers confirming the config wires the IDE to the same `/mcp` endpoint, but the bridge from the IDE's user-facing affordance to that endpoint requires manual confirmation per release. Manual checklist lives at `plugin/tests/MANUAL_CHECKLIST.md` (todo: not yet populated for cursor/copilot/codex flows).

- **Outcome 3 — all IDEs — `manual-checklist`**: Same constraint as Outcome 2. The resolver agent is invocable only from inside a running IDE process. The shared MCP `resolve_annotation` tool IS exercised in CI (server-side); the per-IDE _agent invocation_ requires a human sitting at the IDE.

- **Outcome 4 happy & opt-out — Cursor / Copilot — `manual-checklist`**: Cursor and Copilot reach the daemon transitively through `havi mcp-bridge` (the stdio bridge their `mcp.json` entries point at). The opt-out env var `HAVI_NO_AUTO_REVIVE` is implemented inside the bridge, and that implementation IS covered by the `codex-linux` and `claude-linux` auto-CI jobs. The per-IDE cells are downgraded to `manual-checklist` because the auto-CI tests exercise the bridge directly rather than from a live Cursor/Copilot stdio handshake — i.e. the same code path, but not the same caller. A regression in the bridge would surface in the Codex/Claude auto-CI jobs before reaching Cursor or Copilot.

## Updating this file

When a new outcome is added to the epic, add one row per outcome and re-justify every non-`auto-CI` cell. When a `manual-checklist` cell becomes automated, change its status and remove its justification. CI workflow runs on changes to this file so the matrix and the jobs that link to it stay in sync.
