## Epic Summary — ann-gvd: Auto-context capture in Chrome extension

### What Changed
Annotations now passively capture console errors, failed network requests, web vitals (LCP/CLS/INP), and page metadata alongside the developer's manual comment and screenshot. All context is attached as W3C TextualBody entries with `purpose: "describing"` and `x:role` discriminators.

### Files changed
| File | Change |
|------|--------|
| `extension/src/content/context-collector.js` | NEW — main-world collector (console, network, vitals, CustomEvent bridge) |
| `extension/manifest.json` | Added second content_scripts entry (MAIN world, document_start) |
| `extension/src/content/content.js` | Added getCollectedContext(), getPageMetadata(), buffer size init; made submitAnnotation async; wired context bodies into body[] |

### Architecture
```
PAGE (main world)                         CONTENT SCRIPT (isolated world)
context-collector.js (document_start)     content.js (document_idle)
  console.error/warn → ring buffer          getCollectedContext() ─500ms timeout─→ null
  fetch/XHR 4xx/5xx → ring buffer           getPageMetadata() ─DOM queries─→ obj
  PerformanceObserver → latest values            │
       │                                         ▼
  CustomEvent: __ann_context_response     submitAnnotation() body[] +=
       └──────────────────────────────→     x:role "console-errors"
                                            x:role "network-errors"
                                            x:role "web-vitals"
                                            x:role "page-metadata"
```

### Key User Flows
1. **Capture with errors**: Navigate to page with console errors → Ctrl+Shift+A → select region → save → annotation body[] includes console-errors and page-metadata entries
2. **Capture with failed API**: Page makes 500 request → capture → body[] includes network-errors entry
3. **Clean page capture**: No errors → capture → only comment + image bodies (no empty context entries)
4. **CSP-blocked page**: Main-world script blocked → getCollectedContext() times out → annotation saved normally without context

### How to Test Manually
1. Load extension unpacked in Chrome
2. Navigate to a page, open devtools console, run `console.error('test error')`
3. Capture annotation with Ctrl+Shift+A
4. Check annotation via `GET /api/annotations/:id` — verify body[] contains x:role entries
