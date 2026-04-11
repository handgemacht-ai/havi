# Chrome Extension

Manifest V3 Chrome extension for capturing visual annotations from any web page.

## Architecture

- **Content script** (`src/content/`): Injected into pages. Handles capture activation, region selection, drawing tools, screenshot capture.
- **Background service worker** (`src/background/`): Lifecycle management, `chrome.tabs.captureVisibleTab`, API communication with Go server.
- **Side panel** (`src/sidepanel/`): Annotation list, filtering, CRUD operations.
- **Shared lib** (`src/lib/`): Utilities shared across contexts.

## Conventions

- No build step — plain JS, loaded unpacked via `chrome://extensions`
- Do not introduce a bundler or build step
- Fabric.js for canvas markup (rect, arrow, text, highlight)
- Cropper.js for region selection with resize handles

## Communication

- Content script communicates with background via `chrome.runtime.sendMessage`
- API base URL configured in background service worker, defaults to `http://localhost:8090`
- API base URL stored in `chrome.storage.sync`

## Loading

1. Open `chrome://extensions`
2. Enable "Developer mode"
3. Click "Load unpacked"
4. Select the `extension/` directory
