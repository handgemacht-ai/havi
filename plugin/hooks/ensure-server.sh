#!/usr/bin/env bash
# SessionStart hook: revive the havi daemon if its PID file is missing or stale.
# Quiet on success — never blocks session start.
set -eu

DATA_DIR="${CLAUDE_PLUGIN_DATA:-$HOME/.havi}"
PID_FILE="${DATA_DIR}/havi.pid"

# Drain stdin (SessionStart payload) so the writer doesn't block.
cat >/dev/null 2>&1 || true

# If havi isn't installed, the user hasn't completed /havi:havi-setup yet — skip.
command -v havi >/dev/null 2>&1 || exit 0

if [ -f "$PID_FILE" ]; then
  pid=$(cat "$PID_FILE" 2>/dev/null || echo "")
  if [ -n "$pid" ] && kill -0 "$pid" 2>/dev/null; then
    exit 0
  fi
fi

HAVI_DATA_DIR="$DATA_DIR" havi serve --daemon >/dev/null 2>&1 || true
