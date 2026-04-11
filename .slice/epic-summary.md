## Epic Summary — ann-771: Go Server Foundation

### Overview
Complete Go HTTP server implementing the annotation platform REST API. 4-layer architecture: Controller → Service → Repo → DB, with model types split into domain, W3C contract, and DB record structs.

### Architecture
```
cmd/server/main.go          → entrypoint, env config, graceful shutdown
internal/controller/         → HTTP handlers (7 endpoints + health)
internal/middleware/         → CORS middleware
internal/webhook/            → fire-and-forget webhook
internal/service/            → business logic, W3C envelope construction
internal/repo/               → AnnotationRepo interface + PostgresRepo
internal/db/                 → pgxpool connection + migration runner
internal/model/              → domain entity, W3C DTOs, DB records, errors
```

### Endpoints
| Method | Path | Status | Purpose |
|--------|------|--------|---------|
| POST | /api/annotations | 201 | Create annotation (multipart) |
| GET | /api/annotations | 200 | List with filters |
| GET | /api/annotations/{id} | 200 | Get single |
| GET | /api/annotations/{id}/image | 200 | Get screenshot |
| PUT | /api/annotations/{id} | 200 | Update annotation |
| DELETE | /api/annotations/{id} | 204 | Delete annotation |
| POST | /api/annotations/{id}/resolve | 200 | Resolve with metadata |
| GET | /health | 200 | Health check |

### Key User Flows
1. **Create annotation**: POST multipart with JSON + optional PNG → 201 with W3C envelope, denormalized fields, image URL
2. **Browse annotations**: GET with ?domain=&state= filters → paginated list with total count
3. **Resolve annotation**: POST resolve with metadata → state changes to "resolved", conflict if already resolved
4. **Full lifecycle**: Create → List → Get → Update → Resolve → Delete

### Dependencies
- `github.com/jackc/pgx/v5` — Postgres driver + connection pool
- `github.com/google/uuid` — UUID generation
- Go 1.24, standard library `net/http`

### Configuration
| Env Var | Required | Default | Purpose |
|---------|----------|---------|---------|
| SERVER_DB_URL | yes | — | Postgres connection string |
| SERVER_PORT | no | 8090 | HTTP listen port |
| CORS_ORIGINS | no | chrome-extension://*,http://localhost:* | Allowed CORS origins |
| WEBHOOK_URL | no | — | Webhook target URL |
