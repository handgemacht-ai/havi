#!/usr/bin/env bash
# SessionStart hook: revive the havi daemon if its PID file is missing or stale,
# and export ANNOTATION_SERVER_URL into the Claude env file so MCP interpolation works.
# Quiet on success — never blocks session start.
set -eu

MANAGED_BLOCK_START="# >>> havi-server >>>"
MANAGED_BLOCK_END="# <<< havi-server <<<"
DATA_DIR="${HOME}/.havi"
PID_FILE="${DATA_DIR}/havi.pid"
DEFAULT_URL="http://localhost:8090"

# Drain stdin (SessionStart payload) so the writer doesn't block.
cat >/dev/null 2>&1 || true

# Try to revive the daemon if havi is installed but not running.
if command -v havi >/dev/null 2>&1; then
  needs_start=1
  if [ -f "$PID_FILE" ]; then
    pid=$(cat "$PID_FILE" 2>/dev/null || echo "")
    if [ -n "$pid" ] && kill -0 "$pid" 2>/dev/null; then
      needs_start=0
    fi
  fi
  if [ "$needs_start" = "1" ]; then
    havi serve --daemon >/dev/null 2>&1 || true
  fi
fi

# Append managed env block.
[ -n "${CLAUDE_ENV_FILE:-}" ] || exit 0

mkdir -p "$(dirname "$CLAUDE_ENV_FILE")"

TMP_FILE=$(mktemp "${CLAUDE_ENV_FILE}.havi.XXXXXX")
PRESERVED_FILE=$(mktemp "${CLAUDE_ENV_FILE}.havi.preserved.XXXXXX")
trap 'rm -f "$TMP_FILE" "$PRESERVED_FILE"' EXIT

if [ -f "$CLAUDE_ENV_FILE" ]; then
  awk -v start="$MANAGED_BLOCK_START" -v end="$MANAGED_BLOCK_END" '
    $0 == start { skip = 1; next }
    $0 == end { skip = 0; next }
    skip != 1 { print }
  ' "$CLAUDE_ENV_FILE" > "$PRESERVED_FILE"
else
  : > "$PRESERVED_FILE"
fi

server_url="${ANNOTATION_SERVER_URL:-$DEFAULT_URL}"

{
  if [ -s "$PRESERVED_FILE" ]; then
    cat "$PRESERVED_FILE"
    printf '\n'
  fi
  printf '%s\n' "$MANAGED_BLOCK_START"
  printf 'export ANNOTATION_SERVER_URL=%q\n' "$server_url"
  printf '%s\n' "$MANAGED_BLOCK_END"
} > "$TMP_FILE"

chmod 600 "$TMP_FILE"
mv "$TMP_FILE" "$CLAUDE_ENV_FILE"
rm -f "$PRESERVED_FILE"
