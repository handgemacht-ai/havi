## Epic Summary — ann-wfc: Hook System

### What Changed
The Chrome extension now fetches `/__annotation_context` from the current page's dev server during annotation capture and passes the project context through to the Go server, which stores it in denormalized columns and W3C body.

### Data Flow
```
Content Script → fetch /__annotation_context (500ms timeout)
  → separate known (worktree/branch/project/commit/port) from unknown fields
  → unknown → TextualBody in W3C body
  → known → hookContext in message to background

Background Worker → append hookContext fields as FormData entries
  → POST /api/annotations (multipart)

Go Server → read optional form values
  → project/worktree/branch → denormalized SQL columns
  → commit/port → TextualBody with x:role:"hook-context" in W3C body
```

### Files Changed
| File | Change |
|------|--------|
| `extension/src/content/content.js` | Added fetchHookContext(), made submitAnnotation async |
| `extension/src/background/background.js` | Append hookContext fields to FormData |
| `server/internal/model/w3c.go` | Added Format, XRole to W3CBody |
| `server/internal/controller/annotations.go` | Read optional form fields |
| `server/internal/service/annotation.go` | Added ContextFields, populate columns + hook body |
| `server/integration_test.go` | 3 new tests + createAnnotationWithContext helper |

### Key User Flows
1. **With hook endpoint**: Capture → hook context fetched → denormalized columns populated, commit/port in W3C body
2. **Without hook endpoint**: Capture → fetch fails silently → annotation created normally with empty fields
3. **Partial context**: Only some fields provided → those columns populated, no hook-context body if commit/port absent

### Acceptance Criteria Status
- [x] Dev server with middleware → columns populated from hook response
- [x] Dev server without middleware → annotation created normally, empty fields
- [x] Timeout graceful degradation (500ms AbortController)
- [x] Backward compatible (no hook context = same behavior as before)
