---
name: api-contract
description: Product knowledge about the annotation platform API surface, data model, and design decisions
---

# API Contract — Product Skill

## What Exists

Seven REST endpoints under `/api/annotations`:

| Method | Path | Purpose |
|--------|------|---------|
| POST | `/api/annotations` | Create (multipart: JSON + image) |
| GET | `/api/annotations` | List with query filters |
| GET | `/api/annotations/:id` | Get single |
| GET | `/api/annotations/:id/image` | Get screenshot (image/png) |
| PUT | `/api/annotations/:id` | Partial update |
| DELETE | `/api/annotations/:id` | Delete |
| POST | `/api/annotations/:id/resolve` | Resolve with metadata |

All responses use `{ "data": ... }` envelope for single resources, `{ "data": [...], "meta": { "count": N } }` for collections, `{ "error": { "code": "...", "message": "..." } }` for errors.

Create endpoint accepts `multipart/form-data` with an `annotation` JSON part and optional `image` file part.

## Data Model

Two Postgres tables:

**annotations**: `id` UUID PK, `project`/`domain`/`worktree`/`branch`/`state`/`motivation`/`creator` TEXT with defaults, `annotation` JSONB (W3C envelope), `resolution` JSONB, `created_at`/`updated_at` TIMESTAMPTZ.

**annotation_images**: `annotation_id` UUID PK FK (1:1 with annotations, CASCADE delete), `image_data` BYTEA, `content_type` TEXT, `size_bytes` INTEGER, `created_at` TIMESTAMPTZ.

Indexes on: domain, worktree, branch, state, motivation, creator, created_at DESC.

## W3C Web Annotation

The `annotation` JSONB column stores the canonical W3C Web Annotation envelope with `@context`, `id`, `type`, `motivation`, `created`, `modified`, `creator`, `body` array, and `target` object.

Body types: `TextualBody` (commenting/describing), `Image` (screenshot URL).
Selector types: `CssSelector`, `FragmentSelector` (xywh=), `SvgSelector` (drawn markup).
Motivation values: `commenting`, `highlighting`, `describing`.

SQL columns are denormalized copies for indexed queries only.

## Design Decisions

- JSONB is canonical — SQL columns are denormalized for query performance
- No viewport SQL column — extracted from JSONB at query time
- `annotation_id` as PK on images enforces 1:1 without extra UUID
- `project DEFAULT ''` — populated by hook system when configured
- `resolution` is open-ended JSONB — accepts any metadata (commit, PR, bead ID)
- Server auto-populates: `id`, `created`, `modified`, image body `id`

## Boundaries

- No authentication (out of scope for MVP)
- No full-text search (agent filters client-side)
- No bulk operations
- No API versioning
- No OpenAPI/Swagger spec — API.md is the contract

## Extension Points

- `resolution` JSONB accepts arbitrary metadata for future integrations
- `describing` motivation bodies carry machine-generated context (console, network, vitals)
- Hook system (planned) can populate `project`, `worktree`, `branch` via a dev server endpoint
- Channel notifications fan out to every connected MCP session as `notifications/claude/channel`
