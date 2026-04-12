# Changelog

All notable changes to this project will be documented in this file.
The format is based on [Keep a Changelog](https://keepachangelog.com/).

## [0.3.0] - 2026-04-12

### Added

#### MCP Server
- MCP endpoint at `/mcp` with HTTP Streamable transport (ann-bgn.1)
- `list_annotations` tool with all ListFilters fields: domain, worktree, branch, state, motivation, viewport, creator, limit, offset (ann-bgn.1)
- `get_annotation_image` tool returning base64-encoded screenshot via ImageContent (ann-bgn.1)
- `resolve_annotation` tool with optional metadata map (ann-bgn.1)
- MCP module wired into existing HTTP server alongside REST routes (ann-bgn.2)
- `.mcp.json` at project root for Claude Code auto-discovery (ann-bgn.2)
- 6 MCP integration tests covering initialize, tool listing, filtering, resolve, errors (ann-bgn)

#### Chrome Extension — DOM Element Picker
- "Pick element" button in side panel for DOM-based annotation selection
- Hover highlight overlay (indigo) tracking element bounding rects
- CSS selector generation via css-selector-generator (UMD, 10.6 KB)
- Element text capture as W3C TextualBody with purpose "describing"
- CssSelector stored in annotation target.selector array
- Screenshot auto-cropped to selected element's bounding rect
- Hint bar ("Click an element to annotate — Esc to cancel")

#### Chrome Extension — Capture UX
- Capture button shows "Cancel capture" while active (both modes)
- Cancel from side panel sends cancel-capture to content script
- Error feedback via sendResponse (replaces silent failures)
- Crosshair cursor during markup drawing (rect, arrow, highlight)
- I-beam cursor for text tool

#### Side Panel — Selector Visualization
- Element text shown in expanded card detail (grey block, truncated at 200 chars)
- CSS selector shown in expanded card detail (indigo monospace)

### Fixed
- start-capture-from-panel handler now returns true (keeps message channel open)
- startCaptureInTab throws on invalid URLs instead of silently returning
- Content script responds with error when capture already in progress (prevents stuck UI)
- Ping-retry after content script injection to prevent race condition
- Cancel buttons fall back to reset state when content script is unreachable
- Toast shows red error styling for failures instead of green success icon

### Dependencies
- github.com/modelcontextprotocol/go-sdk v1.5.0
- css-selector-generator v3.9.1 (vendored UMD build)

## [0.2.0] - 2026-04-11

### Added

#### Chrome Extension — API Wiring
- Background service worker API client with 6 message handlers: create (multipart), list, get, update, delete, health check (ann-qnq.1)
- Data URL to Blob conversion for screenshot upload (ann-qnq.1)
- Error handling with timeouts (10s for create, 5s for others) (ann-qnq.1)
- Side panel annotation list with domain-scoped fetching (ann-qnq.2)
- Annotation card UI: thumbnail, comment preview, state/viewport/time chips (ann-qnq.2)
- Expand/collapse detail view with full screenshot and metadata (ann-qnq.2)
- Inline comment editing with save/cancel (ann-qnq.2)
- Delete with inline confirmation dialog (ann-qnq.2)
- State filter bar: All / Open / Resolved (ann-qnq.2)
- Auto-refresh on new annotation via message passing (ann-qnq.2)
- Connection status indicator with 30s health check polling (ann-qnq.2)
- Disconnect banner when server is unreachable (ann-qnq.2)
- `host_permissions` for localhost and 127.0.0.1 (ann-qnq.1)

#### Testing
- E2E smoke test script (`test-e2e.sh`) with 14 curl assertions covering full lifecycle (ann-qnq.3)

## [0.1.0] - 2026-04-11

### Added

#### Chrome Extension
- Chrome Manifest V3 extension foundation (manifest.json, background service worker)
- Keyboard shortcut activation (Ctrl+Shift+A / Cmd+Shift+A)
- Capture overlay with Cropper.js region selection and resize handles
- Fabric.js markup canvas with drawing tools (rectangle, arrow, highlight, text)
- Color picker with 6 preset colors
- Undo support (Ctrl+Z)
- Screenshot compositing (page screenshot + markup as composite PNG)
- Comment input panel with thumbnail preview
- W3C Web Annotation data packaging (FragmentSelector, SvgSelector, HttpRequestState)
- Success toast notification
- ESC to cancel at any stage
- Side panel with empty state and server URL configuration
- Vendored Fabric.js v6.4.3 and Cropper.js v1.6.2

#### Go Server
- Database layer with pgxpool connection management and automatic migration runner (ann-771.1)
- Domain model types: entity, W3C Web Annotation DTOs, DB record structs, typed error constants (ann-771.1)
- AnnotationRepo interface and PostgresRepo implementation with 7 query methods (ann-771.2)
- Service layer with W3C envelope construction, validation, denormalization, resolve-conflict (ann-771.3)
- REST controller with 7 endpoints: create, list, get, get-image, update, delete, resolve (ann-771.4)
- Health check endpoint at GET /health (ann-771.4)
- CORS middleware allowing chrome-extension://* and http://localhost:* origins (ann-771.4)
- Fire-and-forget webhook dispatcher (ann-771.4)
- Graceful shutdown on SIGINT/SIGTERM (ann-771.4)
- 22 scenarigo integration tests against real Postgres

### Dependencies
- github.com/jackc/pgx/v5
- github.com/google/uuid
- github.com/handgemacht-ai/scenarigo v0.6.0
