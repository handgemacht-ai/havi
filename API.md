# Annotation Platform — API Contract

Single source of truth for the REST API, W3C Web Annotation envelope, Postgres schema, and error handling.

## Base URL

```
http://localhost:8090
```

Port configurable via `SERVER_PORT` env var. Worktree offsets apply per `.worktree-ports.json`.

---

## Endpoints

### POST /api/annotations

Create an annotation with optional screenshot.

**Content-Type**: `multipart/form-data`

| Part | Type | Required | Description |
|------|------|----------|-------------|
| `annotation` | JSON | yes | W3C annotation envelope (see [Envelope](#w3c-annotation-envelope)) |
| `image` | file (PNG) | no | Screenshot image |

**Server-populated fields** (overwritten if client sends them):
- `annotation.id` — `urn:uuid:<generated>`
- `annotation.created` — current RFC 3339 timestamp
- `annotation.modified` — current RFC 3339 timestamp
- `body[].id` where type is `Image` — server image URL

**Response**: `201 Created`

```json
{
  "data": {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "annotation": { "...W3C envelope..." },
    "project": "",
    "domain": "localhost:4000",
    "worktree": "",
    "branch": "",
    "state": "open",
    "motivation": "commenting",
    "creator": "maxim",
    "resolution": null,
    "created_at": "2026-04-12T10:30:00Z",
    "updated_at": "2026-04-12T10:30:00Z"
  }
}
```

**Denormalization**: The server extracts `domain` from `target.source`, `motivation` from the top-level field, and `creator` from `creator.name` into indexed SQL columns. `project`, `worktree`, and `branch` come from hook-provided data if present; default to empty string.

**Errors**: `400 validation_error`

---

### GET /api/annotations

List annotations with optional filters.

**Query parameters**:

| Param | Type | Description |
|-------|------|-------------|
| `domain` | string | Filter by domain |
| `worktree` | string | Filter by worktree path |
| `branch` | string | Filter by git branch |
| `state` | string | Filter by state (`open`, `resolved`) |
| `motivation` | string | Filter by motivation (`commenting`, `highlighting`, `describing`) |
| `viewport` | string | Filter by viewport (extracted from annotation JSONB at query time) |
| `creator` | string | Filter by creator name |
| `limit` | integer | Max results (default 50, max 200) |
| `offset` | integer | Pagination offset (default 0) |

**Response**: `200 OK`

```json
{
  "data": [
    {
      "id": "550e8400-e29b-41d4-a716-446655440000",
      "annotation": { "...W3C envelope..." },
      "project": "",
      "domain": "localhost:4000",
      "worktree": "",
      "branch": "",
      "state": "open",
      "motivation": "commenting",
      "creator": "maxim",
      "resolution": null,
      "created_at": "2026-04-12T10:30:00Z",
      "updated_at": "2026-04-12T10:30:00Z"
    }
  ],
  "meta": {
    "count": 42
  }
}
```

`meta.count` is the total matching count (before limit/offset).

---

### GET /api/annotations/:id

Get a single annotation.

**Response**: `200 OK`

```json
{
  "data": {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "annotation": { "...W3C envelope..." },
    "project": "",
    "domain": "localhost:4000",
    "worktree": "",
    "branch": "",
    "state": "open",
    "motivation": "commenting",
    "creator": "maxim",
    "resolution": null,
    "created_at": "2026-04-12T10:30:00Z",
    "updated_at": "2026-04-12T10:30:00Z"
  }
}
```

**Errors**: `404 not_found`

---

### GET /api/annotations/:id/image

Get the screenshot for an annotation.

**Response**: `200 OK`

```
Content-Type: image/png
Content-Length: <size_bytes>

<binary PNG data>
```

**Errors**: `404 not_found` (annotation or image does not exist)

---

### PUT /api/annotations/:id

Update an annotation. Partial update — only provided fields are changed.

**Content-Type**: `application/json`

```json
{
  "annotation": {
    "body": [
      {
        "type": "TextualBody",
        "value": "Updated comment text",
        "purpose": "commenting"
      }
    ]
  }
}
```

The server merges the provided `annotation` fields into the existing W3C envelope. The `modified` timestamp is updated automatically. Denormalized SQL columns (`motivation`, `creator`) are re-extracted if relevant fields changed.

**Response**: `200 OK`

```json
{
  "data": {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "annotation": { "...updated W3C envelope..." },
    "project": "",
    "domain": "localhost:4000",
    "worktree": "",
    "branch": "",
    "state": "open",
    "motivation": "commenting",
    "creator": "maxim",
    "resolution": null,
    "created_at": "2026-04-12T10:30:00Z",
    "updated_at": "2026-04-12T10:35:00Z"
  }
}
```

**Errors**: `400 validation_error`, `404 not_found`

---

### DELETE /api/annotations/:id

Delete an annotation and its associated image.

**Response**: `204 No Content`

**Errors**: `404 not_found`

---

### POST /api/annotations/:id/resolve

Mark an annotation as resolved with metadata.

**Content-Type**: `application/json`

```json
{
  "resolution": {
    "resolved_by": "claude",
    "commit": "abc1234",
    "pr": "https://github.com/org/repo/pull/42",
    "bead_id": "ann-xyz.1",
    "note": "Fixed button alignment in responsive layout"
  }
}
```

The `resolution` object is open-ended — callers pass whatever metadata is relevant (commit hash, PR link, bead ID, free-form note). The server stores the entire object as JSONB and sets `state` to `"resolved"`.

**Response**: `200 OK`

```json
{
  "data": {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "annotation": { "...W3C envelope..." },
    "project": "",
    "domain": "localhost:4000",
    "worktree": "",
    "branch": "",
    "state": "resolved",
    "motivation": "commenting",
    "creator": "maxim",
    "resolution": {
      "resolved_by": "claude",
      "commit": "abc1234",
      "pr": "https://github.com/org/repo/pull/42",
      "bead_id": "ann-xyz.1",
      "note": "Fixed button alignment in responsive layout"
    },
    "created_at": "2026-04-12T10:30:00Z",
    "updated_at": "2026-04-12T11:00:00Z"
  }
}
```

**Errors**: `400 validation_error` (empty resolution), `404 not_found`, `409 conflict` (already resolved)

---

### GET /api/settings/channel-mode

Get the current channel push mode.

**Response**: `200 OK`

```json
{
  "data": {
    "mode": "auto"
  }
}
```

`mode` is either `"auto"` (webhooks fire on create) or `"deferred"` (webhooks only fire via batch push).

---

### PUT /api/settings/channel-mode

Set the channel push mode.

**Content-Type**: `application/json`

```json
{
  "mode": "deferred"
}
```

**Response**: `200 OK`

```json
{
  "data": {
    "mode": "deferred"
  }
}
```

**Errors**: `400 validation_error` (mode must be `"auto"` or `"deferred"`)

---

### POST /api/channel/push

Batch-push annotations to the configured webhook. Used in `deferred` mode to manually trigger webhook delivery.

**Content-Type**: `application/json`

```json
{
  "annotation_ids": ["550e8400-e29b-41d4-a716-446655440000", "6ba7b810-9dad-11d1-80b4-00c04fd430c8"]
}
```

If `annotation_ids` is empty or omitted, all open annotations are pushed.

**Response**: `200 OK`

```json
{
  "data": {
    "pushed": 2
  }
}
```

Non-existent annotation IDs are silently skipped.

**Errors**: `400 webhook_not_configured` (WEBHOOK_URL env var is not set)

---

## Webhook Payload

When a webhook fires (on create in `auto` mode, or via `/api/channel/push`), the payload is the full `AnnotationResponse` DTO — the same shape returned by `GET /api/annotations/:id` inside the `data` envelope:

```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "annotation": { "...W3C envelope..." },
  "project": "",
  "domain": "localhost:4000",
  "worktree": "",
  "branch": "",
  "state": "open",
  "motivation": "commenting",
  "creator": "maxim",
  "resolution": null,
  "created_at": "2026-04-12T10:30:00Z",
  "updated_at": "2026-04-12T10:30:00Z"
}
```

---

## W3C Annotation Envelope

Every annotation stores a W3C Web Annotation JSON-LD envelope in the `annotation` JSONB column. This is the canonical representation. SQL columns are denormalized copies for query performance.

### Canonical Structure

```json
{
  "@context": "http://www.w3.org/ns/anno.jsonld",
  "id": "urn:uuid:<uuid>",
  "type": "Annotation",
  "motivation": "<motivation>",
  "created": "<RFC 3339>",
  "modified": "<RFC 3339>",
  "creator": {
    "type": "Person",
    "name": "<creator name>"
  },
  "body": [ "...bodies..." ],
  "target": {
    "source": "<page URL>",
    "selector": [ "...selectors..." ],
    "state": { "...state..." }
  }
}
```

### Field Rules

| Field | Provided by | Notes |
|-------|-------------|-------|
| `@context` | server | Always `http://www.w3.org/ns/anno.jsonld` |
| `id` | server | `urn:uuid:<generated>`, overwrites client value |
| `type` | server | Always `Annotation` |
| `motivation` | client | Default `commenting`. Values: `commenting`, `highlighting`, `describing` |
| `created` | server | RFC 3339, set on create |
| `modified` | server | RFC 3339, updated on every mutation |
| `creator` | client | `{ "type": "Person", "name": "..." }`. Name defaults to `anonymous` |
| `body` | client | Array of body objects (see Body Types) |
| `target` | client | Required. Must include `source` |

### Body Types

| Type | Purpose | Structure |
|------|---------|-----------|
| `TextualBody` | `commenting` | User's text comment |
| `TextualBody` | `describing` | Machine-generated context (console errors, network failures, web vitals) |
| `Image` | — | Screenshot URL reference. `id` set by server to `/api/annotations/<uuid>/image` |

### Selector Types

| Type | Usage | Example |
|------|-------|---------|
| `CssSelector` | Element the annotation targets | `{ "type": "CssSelector", "value": "main > .card:nth-child(3)" }` |
| `FragmentSelector` | Region coordinates | `{ "type": "FragmentSelector", "conformsTo": "http://www.w3.org/TR/media-frags/", "value": "xywh=120,340,400,200" }` |
| `SvgSelector` | Drawn markup (arrows, rectangles, highlights) | `{ "type": "SvgSelector", "value": "<svg>...</svg>" }` |

### Target State

Viewport dimensions are recorded in `target.state`:

```json
{
  "type": "HttpRequestState",
  "value": "viewport=375x812"
}
```

---

### Example 1: Text Comment with Screenshot

A developer annotates a misaligned button on a dashboard page.

```json
{
  "@context": "http://www.w3.org/ns/anno.jsonld",
  "id": "urn:uuid:550e8400-e29b-41d4-a716-446655440000",
  "type": "Annotation",
  "motivation": "commenting",
  "created": "2026-04-12T10:30:00Z",
  "modified": "2026-04-12T10:30:00Z",
  "creator": {
    "type": "Person",
    "name": "maxim"
  },
  "body": [
    {
      "type": "TextualBody",
      "value": "Button alignment is off on mobile — shifts 8px left below 375px viewport",
      "purpose": "commenting"
    },
    {
      "type": "Image",
      "id": "http://localhost:8090/api/annotations/550e8400-e29b-41d4-a716-446655440000/image"
    }
  ],
  "target": {
    "source": "http://localhost:4000/dashboard",
    "selector": [
      {
        "type": "FragmentSelector",
        "conformsTo": "http://www.w3.org/TR/media-frags/",
        "value": "xywh=120,340,400,200"
      },
      {
        "type": "CssSelector",
        "value": "main > .dashboard-grid > .card:nth-child(3)"
      }
    ],
    "state": {
      "type": "HttpRequestState",
      "value": "viewport=375x812"
    }
  }
}
```

---

### Example 2: Drawing Markup with SvgSelector

A developer draws an arrow pointing to a layout issue and highlights the affected region.

```json
{
  "@context": "http://www.w3.org/ns/anno.jsonld",
  "id": "urn:uuid:6ba7b810-9dad-11d1-80b4-00c04fd430c8",
  "type": "Annotation",
  "motivation": "highlighting",
  "created": "2026-04-12T11:15:00Z",
  "modified": "2026-04-12T11:15:00Z",
  "creator": {
    "type": "Person",
    "name": "maxim"
  },
  "body": [
    {
      "type": "TextualBody",
      "value": "Overlap between sidebar and main content at this breakpoint",
      "purpose": "commenting"
    },
    {
      "type": "Image",
      "id": "http://localhost:8090/api/annotations/6ba7b810-9dad-11d1-80b4-00c04fd430c8/image"
    }
  ],
  "target": {
    "source": "http://localhost:4000/settings",
    "selector": [
      {
        "type": "FragmentSelector",
        "conformsTo": "http://www.w3.org/TR/media-frags/",
        "value": "xywh=0,100,1024,600"
      },
      {
        "type": "CssSelector",
        "value": ".layout-wrapper"
      },
      {
        "type": "SvgSelector",
        "value": "<svg xmlns=\"http://www.w3.org/2000/svg\" viewBox=\"0 0 1024 600\"><line x1=\"200\" y1=\"100\" x2=\"350\" y2=\"250\" stroke=\"red\" stroke-width=\"3\" marker-end=\"url(#arrow)\"/><rect x=\"340\" y=\"200\" width=\"300\" height=\"150\" fill=\"rgba(255,0,0,0.15)\" stroke=\"red\" stroke-width=\"2\"/><defs><marker id=\"arrow\" viewBox=\"0 0 10 10\" refX=\"10\" refY=\"5\" markerWidth=\"6\" markerHeight=\"6\" orient=\"auto-start-reverse\"><path d=\"M 0 0 L 10 5 L 0 10 z\" fill=\"red\"/></marker></defs></svg>"
      }
    ],
    "state": {
      "type": "HttpRequestState",
      "value": "viewport=1024x768"
    }
  }
}
```

---

### Example 3: Machine-Generated Context (Describing Motivation)

An annotation enriched by automatic context capture — console errors and a failed network request recorded alongside the developer's observation.

```json
{
  "@context": "http://www.w3.org/ns/anno.jsonld",
  "id": "urn:uuid:f47ac10b-58cc-4372-a567-0e02b2c3d479",
  "type": "Annotation",
  "motivation": "describing",
  "created": "2026-04-12T14:00:00Z",
  "modified": "2026-04-12T14:00:00Z",
  "creator": {
    "type": "Person",
    "name": "maxim"
  },
  "body": [
    {
      "type": "TextualBody",
      "value": "The chart fails to render after switching to weekly view",
      "purpose": "commenting"
    },
    {
      "type": "Image",
      "id": "http://localhost:8090/api/annotations/f47ac10b-58cc-4372-a567-0e02b2c3d479/image"
    },
    {
      "type": "TextualBody",
      "value": "TypeError: Cannot read properties of undefined (reading 'map') at ChartRenderer.js:142",
      "purpose": "describing"
    },
    {
      "type": "TextualBody",
      "value": "GET /api/analytics/weekly 500 Internal Server Error (took 2340ms)",
      "purpose": "describing"
    },
    {
      "type": "TextualBody",
      "value": "LCP: 4200ms, CLS: 0.35, FID: 120ms",
      "purpose": "describing"
    }
  ],
  "target": {
    "source": "http://localhost:4000/analytics",
    "selector": [
      {
        "type": "FragmentSelector",
        "conformsTo": "http://www.w3.org/TR/media-frags/",
        "value": "xywh=50,200,800,400"
      },
      {
        "type": "CssSelector",
        "value": ".analytics-chart-container"
      }
    ],
    "state": {
      "type": "HttpRequestState",
      "value": "viewport=1440x900"
    }
  }
}
```

---

## Error Responses

All errors use a consistent envelope:

```json
{
  "error": {
    "code": "<error_code>",
    "message": "<human-readable message>"
  }
}
```

### Error Codes

| Code | HTTP Status | When |
|------|-------------|------|
| `validation_error` | 400 | Missing required fields, invalid motivation, malformed JSON, image too large |
| `not_found` | 404 | Annotation or image does not exist |
| `conflict` | 409 | Annotation already resolved (on POST /resolve) |
| `internal_error` | 500 | Unexpected server error |

### Examples

**400 — Validation Error**:
```json
{
  "error": {
    "code": "validation_error",
    "message": "annotation body is required"
  }
}
```

**404 — Not Found**:
```json
{
  "error": {
    "code": "not_found",
    "message": "Annotation not found"
  }
}
```

**409 — Conflict**:
```json
{
  "error": {
    "code": "conflict",
    "message": "Annotation is already resolved"
  }
}
```

**500 — Internal Error**:
```json
{
  "error": {
    "code": "internal_error",
    "message": "An unexpected error occurred"
  }
}
```

---

## Postgres Schema

### annotations

```sql
CREATE TABLE IF NOT EXISTS annotations (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project     TEXT NOT NULL DEFAULT '',
    domain      TEXT NOT NULL DEFAULT '',
    worktree    TEXT NOT NULL DEFAULT '',
    branch      TEXT NOT NULL DEFAULT '',
    state       TEXT NOT NULL DEFAULT 'open',
    motivation  TEXT NOT NULL DEFAULT 'commenting',
    creator     TEXT NOT NULL DEFAULT 'anonymous',
    annotation  JSONB NOT NULL,
    resolution  JSONB,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_annotations_domain ON annotations (domain);
CREATE INDEX IF NOT EXISTS idx_annotations_worktree ON annotations (worktree);
CREATE INDEX IF NOT EXISTS idx_annotations_branch ON annotations (branch);
CREATE INDEX IF NOT EXISTS idx_annotations_state ON annotations (state);
CREATE INDEX IF NOT EXISTS idx_annotations_motivation ON annotations (motivation);
CREATE INDEX IF NOT EXISTS idx_annotations_creator ON annotations (creator);
CREATE INDEX IF NOT EXISTS idx_annotations_created_at ON annotations (created_at DESC);
```

### annotation_images

```sql
CREATE TABLE IF NOT EXISTS annotation_images (
    annotation_id UUID PRIMARY KEY REFERENCES annotations(id) ON DELETE CASCADE,
    image_data    BYTEA NOT NULL,
    content_type  TEXT NOT NULL DEFAULT 'image/png',
    size_bytes    INTEGER NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

### Schema Design Decisions

- **`annotation_id` as PK** on images enforces 1:1 relationship without an extra UUID column
- **`annotation` JSONB** is the canonical W3C envelope; SQL columns are denormalized for indexed queries
- **No `viewport` SQL column** — extracted from annotation JSONB at query time (infrequent filter)
- **`project DEFAULT ''`** — populated by hook system which may not be configured
- **`resolution` JSONB** — open-ended structure for resolve metadata (commit, PR, bead ID, etc.)
- **`creator` SQL column** — denormalized from `annotation.creator.name` for `?creator=` filter
- **`updated_at`** — updated on every mutation (update, resolve)
- **`image_data` BYTEA** — screenshots stored directly in Postgres (adequate for single-team scale)

---

## CORS

The server sets CORS headers on all responses:

| Header | Value |
|--------|-------|
| `Access-Control-Allow-Origin` | `chrome-extension://*`, `http://localhost:*` (configurable via `CORS_ORIGINS` env var) |
| `Access-Control-Allow-Methods` | `GET, POST, PUT, DELETE, OPTIONS` |
| `Access-Control-Allow-Headers` | `Content-Type, Authorization` |
| `Access-Control-Max-Age` | `3600` |

Preflight `OPTIONS` requests return `204 No Content` with the above headers.

---

## Conventions

- **IDs**: UUID v4 (`gen_random_uuid()` in Postgres, `urn:uuid:` prefix in W3C envelope)
- **Timestamps**: RFC 3339 (`2026-04-12T10:30:00Z`)
- **Content-Type**: `application/json` for API requests/responses, `multipart/form-data` for create, `image/png` for image retrieval
- **Envelope**: `{ "data": ... }` for single resource, `{ "data": [...], "meta": { "count": N } }` for collections
- **Nullability**: `resolution` is `null` until resolved; all other fields have defaults
