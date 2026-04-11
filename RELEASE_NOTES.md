# Release Notes

## 2026-04-11

### Extension Core Capture UX (ann-2tl)

A Chrome extension that lets developers capture visual annotations from any web page:

- **Region selection** — dim overlay with drag-to-select and resize handles (Cropper.js)
- **Markup tools** — rectangle, arrow, freehand highlight, and text annotation (Fabric.js v6)
- **Color picker** — 6 preset colors (red, blue, green, yellow, white, black)
- **Screenshot compositing** — composite PNG with page screenshot + markup overlay
- **Comment input** — floating panel with thumbnail preview and text field
- **W3C annotation packaging** — data packaged as W3C Web Annotation
- **Undo support** — Ctrl+Z removes last drawing
- **ESC to cancel** — clean teardown at any stage
- **Side panel** — empty state with keyboard shortcut hint, server URL configuration

### API Wiring & Integration (ann-qnq)

The Chrome extension and Go server are now fully connected:

- **Background API client** — extension background service worker makes real HTTP calls to the server (create with multipart upload, list, get, update, delete, health check)
- **Side panel annotation list** — domain-scoped annotation list with thumbnail cards, expand/collapse detail view, metadata chips (state, viewport, timestamp)
- **Inline editing** — edit annotation comments directly in the side panel
- **Delete with confirmation** — inline confirmation dialog before deletion
- **State filter** — All / Open / Resolved segmented control
- **Auto-refresh** — new annotations appear in the side panel automatically after capture
- **Connection status** — green/gray status dot with health check every 30 seconds, disconnect banner
- **E2E smoke test** — `./test-e2e.sh` validates full annotation lifecycle via curl (14 assertions)

### Annotation Server — Go Foundation (ann-771)

The annotation platform now has a fully functional backend server:

- Create annotations with attached screenshots via the browser extension
- Browse and filter annotations by domain, worktree, branch, state, motivation, viewport, or creator
- Retrieve stored screenshots as PNG images
- Update annotation details or resolve them with metadata
- Health check endpoint for monitoring
- CORS support for Chrome extension origins and localhost
- Webhook notifications on annotation events (fire-and-forget)
- Graceful shutdown on termination signals
