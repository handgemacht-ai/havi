#!/usr/bin/env bash
set -eu

HOOK="$(cd "$(dirname "$0")/.." && pwd)/hooks/ensure-server.sh"
if [ ! -f "$HOOK" ]; then
  echo "FAIL: hook not found at $HOOK" >&2
  exit 1
fi

PASS=0
FAIL=0

run_case() {
  local name="$1"
  shift
  local sandbox
  sandbox=$(mktemp -d)
  local bin="$sandbox/bin"
  local data="$sandbox/data"
  local spawn_log="$sandbox/spawn.log"
  mkdir -p "$bin" "$data"

  cat >"$bin/havi" <<EOF
#!/usr/bin/env bash
echo "\$@" >>"$spawn_log"
exit 0
EOF
  chmod +x "$bin/havi"

  local stderr
  stderr=$(mktemp)

  local exit_code=0
  PATH="$bin:$PATH" CLAUDE_PLUGIN_DATA="$data" "$@" bash "$HOOK" </dev/null 2>"$stderr" >/dev/null || exit_code=$?

  local spawned="no"
  [ -s "$spawn_log" ] && spawned="yes"

  local err_text
  err_text=$(cat "$stderr")

  echo "case=$name exit=$exit_code spawned=$spawned stderr=${err_text//$'\n'/ }"

  CASE_EXIT=$exit_code
  CASE_SPAWNED=$spawned
  CASE_STDERR="$err_text"

  rm -rf "$sandbox" "$stderr"
}

assert_eq() {
  local got="$1" want="$2" what="$3"
  if [ "$got" != "$want" ]; then
    echo "  FAIL ($what): got=$got want=$want" >&2
    return 1
  fi
}

check() {
  if "$@"; then
    PASS=$((PASS + 1))
  else
    FAIL=$((FAIL + 1))
  fi
}

# Case 1: daemon down, no opt-out — must spawn
run_case "auto-revive-default-on"
check assert_eq "$CASE_EXIT" "0" "exit"
check assert_eq "$CASE_SPAWNED" "yes" "spawn"

# Case 2: daemon down, opt-out set — must NOT spawn
run_case "opt-out-honored" env HAVI_NO_AUTO_REVIVE=1
check assert_eq "$CASE_EXIT" "0" "exit"
check assert_eq "$CASE_SPAWNED" "no" "no-spawn"
if ! echo "$CASE_STDERR" | grep -q "HAVI_NO_AUTO_REVIVE"; then
  echo "  FAIL (stderr-msg): expected 'HAVI_NO_AUTO_REVIVE' in stderr, got: $CASE_STDERR" >&2
  FAIL=$((FAIL + 1))
else
  PASS=$((PASS + 1))
fi

# Case 3: idempotent — daemon up (PID file points at this shell), no opt-out, no spawn
run_case_pid_live() {
  local sandbox bin data spawn_log
  sandbox=$(mktemp -d)
  bin="$sandbox/bin"
  data="$sandbox/data"
  spawn_log="$sandbox/spawn.log"
  mkdir -p "$bin" "$data"

  cat >"$bin/havi" <<EOF
#!/usr/bin/env bash
echo "\$@" >>"$spawn_log"
exit 0
EOF
  chmod +x "$bin/havi"

  echo "$$" >"$data/havi.pid"

  local exit_code=0
  PATH="$bin:$PATH" CLAUDE_PLUGIN_DATA="$data" bash "$HOOK" </dev/null >/dev/null 2>&1 || exit_code=$?

  local spawned="no"
  [ -s "$spawn_log" ] && spawned="yes"

  echo "case=idempotent-daemon-up exit=$exit_code spawned=$spawned"
  CASE_EXIT=$exit_code
  CASE_SPAWNED=$spawned
  rm -rf "$sandbox"
}

run_case_pid_live
check assert_eq "$CASE_EXIT" "0" "exit"
check assert_eq "$CASE_SPAWNED" "no" "no-spawn-when-alive"

# Case 4: idempotent under opt-out — daemon up, opt-out set, no spawn, no opt-out notice (early return)
run_case_pid_live_optout() {
  local sandbox bin data spawn_log
  sandbox=$(mktemp -d)
  bin="$sandbox/bin"
  data="$sandbox/data"
  spawn_log="$sandbox/spawn.log"
  mkdir -p "$bin" "$data"

  cat >"$bin/havi" <<EOF
#!/usr/bin/env bash
echo "\$@" >>"$spawn_log"
exit 0
EOF
  chmod +x "$bin/havi"

  echo "$$" >"$data/havi.pid"

  local stderr
  stderr=$(mktemp)
  local exit_code=0
  PATH="$bin:$PATH" CLAUDE_PLUGIN_DATA="$data" HAVI_NO_AUTO_REVIVE=1 bash "$HOOK" </dev/null >/dev/null 2>"$stderr" || exit_code=$?

  local spawned="no"
  [ -s "$spawn_log" ] && spawned="yes"
  local err_text
  err_text=$(cat "$stderr")

  echo "case=idempotent-daemon-up-with-optout exit=$exit_code spawned=$spawned stderr=${err_text//$'\n'/ }"
  CASE_EXIT=$exit_code
  CASE_SPAWNED=$spawned
  CASE_STDERR="$err_text"
  rm -rf "$sandbox" "$stderr"
}

run_case_pid_live_optout
check assert_eq "$CASE_EXIT" "0" "exit"
check assert_eq "$CASE_SPAWNED" "no" "no-spawn-when-alive-optout"
if [ -n "$CASE_STDERR" ]; then
  echo "  FAIL (silent-when-alive): expected no stderr, got: $CASE_STDERR" >&2
  FAIL=$((FAIL + 1))
else
  PASS=$((PASS + 1))
fi

echo
echo "PASS=$PASS FAIL=$FAIL"
[ "$FAIL" -eq 0 ]
