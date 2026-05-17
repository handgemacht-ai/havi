# HAVI

Self-hosted annotation platform. Capture visual + technical observations from any web page during development; route them to active Claude Code sessions for context-rich fixes.

Annotations are stored as [W3C Web Annotations](https://www.w3.org/TR/annotation-model/) (default: SQLite at `~/.havi/havi.db`; opt-in: Postgres) and pushed in real-time into your Claude Code session via MCP.

## Install

### Recommended: Claude Code plugin

```text
# in any Claude Code session
/plugin marketplace add handgemacht-ai/havi
/plugin install havi@havi
/havi-setup
```

Then restart Claude with the plugin loaded:

```bash
claude --dangerously-load-development-channels plugin:havi@havi
```

`/havi-setup` downloads the prebuilt server binary, starts it as a background daemon under `${CLAUDE_PLUGIN_DATA}`, and opens the [Chrome Web Store listing](https://chrome.google.com/webstore/detail/deedaihcndphkolmnfegjfjilcadncil) for the extension. Subsequent sessions auto-revive the daemon via the plugin's `SessionStart` hook.

If `/havi-setup` reports `operation not permitted` on bind, the Claude Code sandbox is blocking port 8090. Either run `havi serve --daemon` once from a terminal outside Claude (the SessionStart hook will keep it alive afterwards), or allow local binding in `.claude/settings.json`:

```json
{ "sandbox": { "network": { "allowLocalBinding": true },
               "excludedCommands": ["havi *"] } }
```

### Manual binary install

```bash
curl -fsSL https://raw.githubusercontent.com/handgemacht-ai/havi/main/scripts/install.sh | sh
havi serve --daemon
curl http://localhost:8090/health
```

The installer fetches the latest release from [GitHub releases](https://github.com/handgemacht-ai/havi/releases), verifies the checksum, and drops the binary into `~/.local/bin/havi` (override with `HAVI_INSTALL_DIR=...`). Builds available for darwin/linux × amd64/arm64.

### Browser extension

Install from the [Chrome Web Store](https://chrome.google.com/webstore/detail/deedaihcndphkolmnfegjfjilcadncil), then click the toolbar icon on any page to start capturing.

## Cursor

[Cursor](https://cursor.com/) reads MCP server definitions from `~/.cursor/mcp.json`. `havi install cursor` adds (or updates) one named key — `mcpServers.havi` — and leaves every other key in the file byte-stable.

```bash
havi install cursor     # add or update mcpServers.havi in ~/.cursor/mcp.json
havi uninstall cursor   # remove mcpServers.havi; unrelated servers stay
```

Re-running `install` is a no-op when the entry already matches. Uninstall deletes the file entirely when havi was the only key; otherwise it preserves every byte outside `mcpServers.havi`. The equivalent stanza, if you prefer to write it yourself, is:

```json
{
  "mcpServers": {
    "havi": {
      "command": "havi",
      "args": ["mcp-bridge"]
    }
  }
}
```

## GitHub Copilot (VS Code)

[GitHub Copilot in VS Code](https://code.visualstudio.com/docs/copilot/customization/mcp-servers) reads MCP server definitions from `.vscode/mcp.json` (workspace-local) or your VS Code user-profile `mcp.json` (`~/.config/Code/User/mcp.json` on Linux, `~/Library/Application Support/Code/User/mcp.json` on macOS). `havi install copilot` writes the workspace file by default; pass `--global` for the user-profile fallback.

```bash
havi install copilot              # writes ./.vscode/mcp.json
havi install copilot --global     # writes the VS Code user-profile mcp.json
havi uninstall copilot            # remove servers.havi from the workspace file
havi uninstall copilot --global   # same, against the user-profile file
```

The writer owns one named key — `servers.havi` — and leaves every other key in the file byte-stable, so unrelated MCP servers (e.g. `github`, `playwright`) and unrelated top-level keys (e.g. `inputs`) survive both install and uninstall untouched. Uninstall deletes the file entirely when havi was the only content; otherwise the rest of the file is preserved byte-for-byte. The equivalent manual stanza is:

```json
{
  "servers": {
    "havi": {
      "command": "havi",
      "args": ["mcp-bridge"]
    }
  }
}
```

## MCP bridge (Codex / stdio-only clients)

Some MCP clients (such as OpenAI Codex CLI) communicate over stdio rather than HTTP. `havi mcp-bridge` is a thin subprocess that reads newline-delimited JSON-RPC frames from stdin, forwards them to the local havi server's `/mcp` endpoint using the Streamable HTTP MCP protocol, and writes each JSON-RPC response back to stdout. On the first request it automatically starts the havi daemon if it is not already running (opt out by setting `HAVI_NO_AUTO_REVIVE=1`).

```bash
havi install codex     # add the managed [[mcp_servers]] entry to ~/.codex/config.toml
havi uninstall codex   # remove it; leaves the rest of the file byte-identical
```

`havi install codex` first probes `codex --version`; if the Codex CLI is not on PATH it exits non-zero without writing. The entry is wrapped in a managed-block comment so re-running is a no-op and unrelated MCP servers in `~/.codex/config.toml` are left untouched. If you prefer to wire it manually, the equivalent stanza is:

```toml
[[mcp_servers]]
name = "havi"
command = "havi"
args  = ["mcp-bridge"]
```

`HAVI_NO_AUTO_REVIVE` is honored by every revival mechanism — the Codex stdio bridge (above) and the Claude Code plugin's `SessionStart` hook. Export it once in your shell to disable all automatic daemon spawning; `havi serve` still works when invoked manually.

## Agent instructions (AGENTS.md)

[`AGENTS.md`](https://agents.md/) is a Linux Foundation standard (December 2025) read by Codex, Cursor, GitHub Copilot, Gemini CLI, Windsurf, and other coding agents. `havi install agents-md` writes a managed block describing the three havi MCP tools (`list_annotations`, `get_annotation_image`, `resolve_annotation`), the git-context preamble agents should run before each call, and how to review and resolve annotations.

```bash
havi install agents-md             # writes ./AGENTS.md in the current project
havi install agents-md --global    # writes ~/.codex/AGENTS.md (user-global fallback)
havi uninstall agents-md           # removes the managed block; rest of file byte-identical
havi uninstall agents-md --global  # same, against the global file
```

The managed block is wrapped in HTML comments so the rest of the file (existing prose, other sections, team conventions) stays byte-identical between runs. Editing inside the block is overwritten on the next `install`; everything outside is preserved.

## Storage

| Path | Purpose |
|------|---------|
| `${HAVI_DATA_DIR:-$HOME/.havi}/havi.db` | SQLite database (default) |
| `${HAVI_DATA_DIR:-$HOME/.havi}/havi.pid` | Daemon PID file |
| `${HAVI_DATA_DIR:-$HOME/.havi}/server.log` | Daemon log |

The Claude plugin sets `HAVI_DATA_DIR=${CLAUDE_PLUGIN_DATA}` so plugin-managed data stays inside the plugin's writable area.

To override the database location directly: `HAVI_DB_URL=/path/to/file.db` or `HAVI_DB_URL=sqlite:///path/to/file.db`.

## Postgres (opt-in)

```bash
just up   # start Postgres via Docker Compose
HAVI_DB_URL=postgres://annotations:dev@localhost:5432/annotations?sslmode=disable havi serve
```

`just down` stops it; `just reset` wipes the volume.

## Ports

| Service | Port | Env var |
|---------|------|---------|
| Go server (HTTP + MCP) | 8090 | `SERVER_PORT` |
| Postgres (opt-in) | 5432 | `DB_PORT` |

## Build from source

```bash
just server       # run the Go server with SQLite at ~/.havi/havi.db
just test         # run all tests
just lint         # golangci-lint
```

## Documentation

- [API.md](API.md) — REST + MCP contract (single source of truth for request/response shapes)
- [ROADMAP.md](ROADMAP.md) — full scope and milestones
- [CHANGELOG.md](CHANGELOG.md) — release notes
- [PRIVACY.md](PRIVACY.md) — data handling
