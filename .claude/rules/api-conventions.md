# REST API Conventions

## Resources

| Method | Path | Purpose |
|--------|------|---------|
| `POST` | `/api/annotations` | Create annotation (multipart: JSON + image) |
| `GET` | `/api/annotations` | List annotations (query param filters) |
| `GET` | `/api/annotations/:id` | Get single annotation |
| `GET` | `/api/annotations/:id/image` | Get screenshot (returns image/png) |
| `PUT` | `/api/annotations/:id` | Update annotation |
| `DELETE` | `/api/annotations/:id` | Delete annotation |
| `POST` | `/api/annotations/:id/resolve` | Resolve with metadata |

## JSON Envelope

Single resource:
```json
{ "data": { ... } }
```

Collection:
```json
{ "data": [ ... ], "meta": { "count": 42 } }
```

Error:
```json
{ "error": { "code": "not_found", "message": "Annotation not found" } }
```

## Content Types

| Context | Content-Type |
|---------|-------------|
| JSON API requests/responses | `application/json` |
| Image upload (create annotation) | `multipart/form-data` |
| Image retrieval | `image/png` |

## Conventions

- Timestamps: RFC 3339 (`2026-04-12T10:30:00Z`)
- IDs: UUID v4
- Filters via query params: `domain`, `worktree`, `branch`, `state`, `motivation`, `viewport`, `creator`, `limit`, `offset`
- CORS: allow `chrome-extension://*` and `http://localhost:*`
