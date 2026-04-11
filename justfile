# Annotation Platform — dev commands

up:
    docker compose up -d

down:
    docker compose down

reset:
    docker compose down -v && docker compose up -d

logs:
    docker compose logs -f

status:
    docker compose ps

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
