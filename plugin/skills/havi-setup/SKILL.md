---
name: havi-setup
description: One-shot install for HAVI on this machine. Downloads the havi server binary, starts it as a background daemon, and opens the Chrome Web Store page so the user can finish by clicking "Add to Chrome".
allowed-tools: Bash
user-invocable: true
---

## HAVI Setup

This skill delegates to `${CLAUDE_PLUGIN_ROOT}/bin/havi-setup`, a single bash script that:

1. Installs the `havi` binary if missing (`scripts/install.sh` from the GitHub repo)
2. Starts the daemon with `HAVI_DATA_DIR=${CLAUDE_PLUGIN_DATA}` so DB, PID, and log live under the plugin's writable data dir
3. Probes `http://localhost:8090/health`
4. Probes the MCP transport (`POST /mcp initialize`)
5. Opens the Chrome Web Store listing for the extension

The script is idempotent — re-running it confirms each step instead of redoing the work.

### Run it

```bash
bash "${CLAUDE_PLUGIN_ROOT}/bin/havi-setup"
```

If the script reports "operation not permitted" on bind, the Claude Code sandbox is blocking port 8090. Two ways to unblock:

- **A.** Run `havi serve --daemon` once from a regular terminal. The plugin's SessionStart hook (`ensure-server.sh`) will then keep it alive across sessions.
- **B.** Run `/sandbox` and enable `allowLocalBinding`, or add to `.claude/settings.json`:
  ```json
  { "sandbox": { "network": { "allowLocalBinding": true },
                 "excludedCommands": ["havi *"] } }
  ```

### After it succeeds

Tell the user:

> Run `/reload-plugins` to reconnect the MCP client to the running daemon, then click **Add to Chrome** on the page that just opened.
