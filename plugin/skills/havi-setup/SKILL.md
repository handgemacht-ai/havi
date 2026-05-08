---
name: havi-setup
description: One-shot install for HAVI on this machine. Downloads the havi server binary, starts it as a background daemon, and opens the Chrome Web Store page so the user can finish by clicking "Add to Chrome".
allowed-tools: Bash, Read
user-invocable: true
---

## HAVI Setup

This skill brings a fresh machine to a working HAVI install in one step. After it succeeds:

- `havi` is installed at `~/.local/bin/havi` (or `/usr/local/bin/havi` if writable).
- The server is running in the background, listening on `localhost:8090`.
- The SQLite database lives at `~/.havi/havi.db`.
- The Chrome Web Store listing is open in the user's browser, ready to install the extension.
- `ANNOTATION_SERVER_URL=http://localhost:8090` is exported into the Claude session env.

The setup is idempotent — re-running it on an already-set-up machine confirms each step instead of redoing the work.

### Step 1: Check if `havi` is installed

```bash
if command -v havi >/dev/null 2>&1; then
  echo "havi already installed: $(havi --version)"
else
  echo "installing havi"
  curl -fsSL https://raw.githubusercontent.com/handgemacht-ai/havi/main/scripts/install.sh | sh
fi
```

If the install script reports that `~/.local/bin` is not on PATH, surface its message to the user verbatim and instruct them to add it to their shell rc file before continuing. Re-check `command -v havi` after that.

### Step 2: Start the daemon

```bash
havi serve --daemon
```

The first run creates `~/.havi/havi.db` and applies the SQLite migrations automatically; the existing `db.Migrate` call inside `havi serve` is idempotent. Subsequent runs detect the live PID at `~/.havi/havi.pid` and exit cleanly.

### Step 3: Probe the health endpoint

Retry up to 5 times with 500ms between attempts:

```bash
for i in 1 2 3 4 5; do
  if curl -fsS http://localhost:8090/health >/dev/null 2>&1; then
    echo "server healthy"
    break
  fi
  sleep 0.5
done
```

If `/health` never responds, read the last 50 lines of `~/.havi/server.log` and surface the error to the user so they can diagnose port conflicts or permission issues. Do not retry indefinitely.

### Step 4: Probe the MCP transport

```bash
curl -fsS -X POST http://localhost:8090/mcp \
  -H "Content-Type: application/json" \
  -H "Accept: application/json, text/event-stream" \
  -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"havi-setup","version":"0.1"}}}' \
  >/dev/null
```

A non-zero exit from this call indicates the MCP transport is not reachable — re-check Step 3 before proceeding.

### Step 5: Open the Chrome Web Store listing

```bash
url="https://chrome.google.com/webstore/detail/deedaihcndphkolmnfegjfjilcadncil"
if command -v open >/dev/null 2>&1; then
  open "$url"
elif command -v xdg-open >/dev/null 2>&1; then
  xdg-open "$url"
else
  echo "open this URL in your browser to install the extension: $url"
fi
```

Tell the user to click **Add to Chrome** on the page that just opened, then come back to the Claude session.

### Step 6: Export `ANNOTATION_SERVER_URL` into the session env

The plugin's MCP config interpolates `${ANNOTATION_SERVER_URL}/mcp`. Write it into the managed block of `${CLAUDE_ENV_FILE}` if that variable is set; otherwise skip silently (the SessionStart `ensure-server.sh` hook re-establishes it on the next session).

Use the same managed-block pattern that `plugin/hooks/collect-env.sh` uses, with markers `# >>> havi-server >>>` / `# <<< havi-server <<<`. If `${CLAUDE_ENV_FILE}` is unset, just `echo` the export so the user can add it manually.

### Step 7: Final report

Print a short banner summarizing the result:

```
HAVI ready
  binary:   ~/.local/bin/havi
  database: ~/.havi/havi.db
  daemon:   pid <PID> on http://localhost:8090
  log:      ~/.havi/server.log
  next:     install the Chrome extension from the page that just opened
```

If any step failed, print the failure cause and the recovery command so the user can run it manually.
