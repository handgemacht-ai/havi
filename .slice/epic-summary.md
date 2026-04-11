## Epic Summary — ann-gex: API Contract & Schema Design

### Overview
This epic produces the foundational API contract that unblocks both the Go server (Epic 2) and Chrome extension (Epic 3) for parallel development.

### Deliverables

| Bead | Output | Description |
|------|--------|-------------|
| ann-gex.1 | API.md | Complete REST API contract with 7 endpoints, 3 W3C examples, error codes, schema, CORS |
| ann-gex.2 | server/migrations/001_create_annotations.sql | Idempotent Postgres migration for annotations + annotation_images |
| ann-gex.3 | .claude/skills/product/api-contract.md | Product skill for agent context |
| ann-gex.3 | .claude/skills/docs-updater.md | Docs maintenance instructions |

### API Surface
- POST /api/annotations (multipart create)
- GET /api/annotations (list with filters)
- GET /api/annotations/:id (get single)
- GET /api/annotations/:id/image (get screenshot)
- PUT /api/annotations/:id (partial update)
- DELETE /api/annotations/:id (delete)
- POST /api/annotations/:id/resolve (resolve with metadata)

### Data Model
- **annotations**: 12 columns, JSONB canonical W3C envelope, 7 indexed SQL columns for queries
- **annotation_images**: 1:1 with annotations via PK FK, BYTEA image storage

### Key Design Decisions
- W3C JSONB is canonical; SQL columns are denormalized for performance
- No viewport SQL column (extracted from JSONB)
- annotation_id as PK on images (enforces 1:1)
- resolution is open-ended JSONB
- project defaults to empty string (hook-dependent)

### Cross-Bead Consistency
- API.md Postgres schema section matches migration SQL exactly
- Product skill accurately summarizes API.md contents
- Docs-updater skill references all files that need coordinated updates
