# Annotation Platform — dev commands
#
# Default storage is SQLite at ~/.havi/havi.db; `just server` works with no infra.
# Postgres is opt-in: run `just up` and set HAVI_DB_URL=postgres://... before `just server`.

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

build-firefox:
    bash scripts/build-firefox.sh

pack:
    google-chrome-stable --pack-extension=extension {{ if path_exists("extension.pem") == "true" { "--pack-extension-key=extension.pem" } else { "" } }} --no-sandbox
    cd extension && python3 -c "import zipfile, pathlib; z=zipfile.ZipFile('../extension.zip','w',zipfile.ZIP_DEFLATED); [z.write(f,f.relative_to('.')) for f in pathlib.Path('.').rglob('*') if f.is_file() and not any(p.startswith('.') for p in f.parts)]; z.close()"
    @ls -lh extension.crx extension.zip

release version:
    #!/usr/bin/env bash
    set -euo pipefail
    version="{{version}}"
    if ! [[ "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
      echo "version must be MAJOR.MINOR.PATCH (got: $version)" >&2
      exit 1
    fi
    if [ -n "$(git status --porcelain extension/manifest.json)" ]; then
      echo "extension/manifest.json has uncommitted changes — refusing to release" >&2
      exit 1
    fi
    python3 -c "
    import json, pathlib
    p = pathlib.Path('extension/manifest.json')
    data = json.loads(p.read_text())
    data['version'] = '$version'
    p.write_text(json.dumps(data, indent=2) + '\n')
    "
    git add extension/manifest.json
    git commit -m "chore(extension): bump to v$version"
    git tag "ext-v$version"
    branch="$(git rev-parse --abbrev-ref HEAD)"
    git push origin "$branch"
    git push origin "ext-v$version"
    echo "Released ext-v$version on $branch"

release-server version:
    #!/usr/bin/env bash
    set -euo pipefail
    version="{{version}}"
    if ! [[ "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
      echo "version must be MAJOR.MINOR.PATCH (got: $version)" >&2
      exit 1
    fi
    if [ -n "$(git status --porcelain server/internal/version/version.go)" ]; then
      echo "server/internal/version/version.go has uncommitted changes — refusing to release" >&2
      exit 1
    fi
    python3 -c "
    import pathlib, re
    p = pathlib.Path('server/internal/version/version.go')
    data = p.read_text()
    new = re.sub(r'var Version = \".*\"', 'var Version = \"$version\"', data, count=1)
    if new == data:
        raise SystemExit('failed to bump version constant')
    p.write_text(new)
    "
    git add server/internal/version/version.go
    git commit -m "chore(server): bump to v$version"
    git tag "server-v$version"
    branch="$(git rev-parse --abbrev-ref HEAD)"
    git push origin "$branch"
    git push origin "server-v$version"
    echo "Released server-v$version on $branch"
