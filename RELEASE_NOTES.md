# Release Notes — Extension Core Capture UX (ann-2tl)

## What's New

### Visual Annotation Capture (Ctrl+Shift+A)

A Chrome extension that lets developers capture visual annotations from any web page:

- **Region selection** — dim overlay with drag-to-select and resize handles (Cropper.js)
- **Markup tools** — rectangle, arrow, freehand highlight, and text annotation (Fabric.js v6)
- **Color picker** — 6 preset colors (red, blue, green, yellow, white, black)
- **Screenshot compositing** — composite PNG with page screenshot + markup overlay
- **Comment input** — floating panel with thumbnail preview and text field
- **W3C annotation packaging** — data packaged as W3C Web Annotation with FragmentSelector, SvgSelector, and HttpRequestState
- **Undo support** — Ctrl+Z removes last drawing
- **ESC to cancel** — clean teardown at any stage

### Side Panel

- Empty state with keyboard shortcut hint
- Server URL configuration (stored in chrome.storage.sync)
- Settings toggle with smooth slide animation

## Architecture

Chrome Manifest V3 extension with three layers:
- **Content script** — capture flow state machine (idle → capturing → selected → markup → commenting)
- **Background service worker** — captureVisibleTab, chrome.storage, message routing
- **Side panel** — settings and annotation list shell (list populated in Epic 4)

Vendored libraries: Fabric.js v6.4.3, Cropper.js v1.6.2. No build step.

## Known Limitations

- Annotations logged to console only (server API integration in Epic 4)
- Status dot in side panel is visual-only (no health check)
- Icons are placeholder
