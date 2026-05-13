#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

SRC="$ROOT/extension"
OVERLAY="$ROOT/firefox"
OUT="${1:-$ROOT/firefox-build}"

if [ ! -d "$SRC" ]; then
  echo "extension source not found at $SRC" >&2
  exit 1
fi
if [ ! -f "$OVERLAY/manifest.json" ]; then
  echo "firefox overlay manifest not found at $OVERLAY/manifest.json" >&2
  exit 1
fi

rm -rf "$OUT"
mkdir -p "$OUT"

rsync -a \
  --exclude manifest.json \
  --exclude CLAUDE.md \
  --exclude .prettierrc \
  "$SRC/" "$OUT/"

rsync -a \
  --exclude CLAUDE.md \
  "$OVERLAY/" "$OUT/"

BG="$OUT/src/background/background.js"
if ! grep -q 'firefox-build:strip-start' "$BG"; then
  echo "expected firefox-build:strip markers in $BG (source background.js out of sync)" >&2
  exit 1
fi
perl -i -0777 -pe 's{\s*// firefox-build:strip-start.*?// firefox-build:strip-end\n}{}s' "$BG"
if grep -q 'chrome\.sidePanel' "$BG"; then
  echo "chrome.sidePanel still referenced in $BG after strip" >&2
  exit 1
fi

echo "built firefox extension: $OUT"
