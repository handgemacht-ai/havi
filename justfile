# Annotation Platform — dev commands

set dotenv-load

up:
    docker compose up -d

down:
    docker compose down

reset:
    docker compose down -v && docker compose up -d

logs:
    docker compose logs -f

status:
    #!/usr/bin/env bash
    set -euo pipefail
    rig_root="{{justfile_directory()}}"
    if [ -n "${WORKSPACE_ROOT:-}" ]; then
      ws="$WORKSPACE_ROOT"
    elif [ -f "$rig_root/.worktree.env" ]; then
      main="$(grep '^WORKTREE_MAIN_PATH=' "$rig_root/.worktree.env" | head -1 | cut -d= -f2)"
      ws="$(realpath "$main/..")"
    else
      ws="$(realpath "$rig_root/..")"
    fi
    if [ ! -x "$ws/scripts/runtime/just-status.sh" ]; then
      echo "just-status.sh not found at $ws/scripts/runtime/" >&2
      exit 1
    fi
    exec bash "$ws/scripts/runtime/just-status.sh" "$rig_root"

server:
    cd server && go run ./cmd/server

channel:
    cd channel && bun run src/server.ts

db-migrate:
    @echo "Not implemented — will be added in Epic 2"

db-reset:
    @echo "Not implemented — will be added in Epic 2"

lint:
    cd server && golangci-lint run --build-tags scenario

fmt:
    cd server && gofmt -w .

test: test-server

test-server:
    cd server && go test -tags scenario -count=1 ./...

pack:
    google-chrome-stable --pack-extension=extension {{ if path_exists("extension.pem") == "true" { "--pack-extension-key=extension.pem" } else { "" } }} --no-sandbox
    cd extension && python3 -c "import zipfile, pathlib; z=zipfile.ZipFile('../extension.zip','w',zipfile.ZIP_DEFLATED); [z.write(f,f.relative_to('.')) for f in pathlib.Path('.').rglob('*') if f.is_file() and not any(p.startswith('.') for p in f.parts)]; z.close()"
    @ls -lh extension.crx extension.zip
