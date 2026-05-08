# W3C Web Annotation Conventions

All annotations use the W3C Web Annotation data model. This file is the vocabulary reference for all sessions.

## Envelope Structure

Every annotation stored in the `annotation` column (Postgres JSONB / SQLite TEXT with `json_valid` check) must follow this structure:

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
      "value": "Button alignment is off on mobile",
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
      },
      {
        "type": "SvgSelector",
        "value": "<svg>...</svg>"
      }
    ],
    "state": {
      "type": "HttpRequestState",
      "value": "viewport=375x812"
    }
  }
}
```

## Body Types

| Type | Purpose | Usage |
|------|---------|-------|
| `TextualBody` | `commenting` | User's text comment |
| `Image` | — | Screenshot URL reference |
| `TextualBody` | `describing` | Console errors, network failures, web vitals |

## Selector Types

| Type | Usage |
|------|-------|
| `CssSelector` | Element the annotation targets |
| `FragmentSelector` | Region coordinates (`xywh=x,y,w,h`) |
| `SvgSelector` | Drawn markup (arrows, rectangles, highlights) |

## Motivation Values

- `commenting` (default) — user observation or feedback
- `highlighting` — visual emphasis without comment
- `describing` — machine-generated context (console, network, vitals)

## Rules

- Always store the full W3C envelope in the `annotation` column (Postgres JSONB / SQLite TEXT)
- Never flatten W3C fields into top-level SQL columns
- Use W3C field names in API responses — do not invent aliases
- Indexed SQL columns (project, domain, worktree, branch, state, motivation) are denormalized copies for query performance only
