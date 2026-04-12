---
version: "1.0"
created: 2026-04-12
status: active
scope: "Day 2 — Sunday April 12, 2026"
sessions: 3
---

# Sunday Implementation Plan — Annotation Platform

## Status After Saturday

All Saturday exit criteria met. The following is shipped on `main`:

- Go server: full CRUD, CORS, webhook dispatcher, health check, graceful shutdown (22 integration tests)
- Chrome extension: capture (region + element picker), Fabric.js markup, comment, side panel with list/CRUD/filters
- API wiring: extension ↔ server working end-to-end
- MCP server: `/mcp` endpoint with `list_annotations`, `get_annotation_image`, `resolve_annotation` (6 tests)
- Claude Code plugin: `.mcp.json`, `collect-env.sh` hook, `setup-hooks` skill (Next.js/Vite/Phoenix), `annotation-resolver` agent

## Sunday Goal

**Success criteria from ROADMAP**: Live demo by 4pm — capture a UI bug in Chrome with markup and comment → W3C annotation stored in Postgres → pushed via channel into active Claude Code session → agent reads screenshot and structured context → generates a fix. End-to-end in under 60 seconds.

## Gap Analysis

| # | Component | Roadmap section | Critical path? |
|---|-----------|-----------------|----------------|
| 1 | Channel server (Bun) | Claude Code Channel Plugin | **Yes** — demo blocker |
| 2 | Hook system in extension | Hook System — Project-Specific Enrichment | No — enriches context but demo works without it |
| 3 | Auto-context capture | Chrome Extension — Automatic Context Capture | No — enriches context but demo works without it |
| 4 | Responsive viewport presets | Chrome Extension — Responsive Viewport Annotation | No — independent feature |
| 5 | End-to-end demo flow | Success Criteria | **Yes** — the deliverable |

## Cut Decision

**Responsive viewports (#4) are cut from Sunday scope.** Reason: iframe-based viewport simulation is a self-contained feature with no dependency on the channel or demo flow. It adds complexity without contributing to the success criteria. Ship it post-hackathon.

## Execution Model

Two phases. Phase 1 builds the channel (critical path) in parallel with extension enrichment. Phase 2 integrates everything and runs the demo.

```
Time         Session A (Channel)       Session B (Extension)       Session C (Integration)
──────────── ───────────────────────── ─────────────────────────── ──────────────────────────
                                                                   
 Phase 1     ┌───────────────────────┐ ┌─────────────────────────┐ ┌────────────────────────┐
 (3-4 hrs)   │ Epic 5: Channel Server│ │ Epic 6: Extension       │ │ Epic 7: Hook System    │
             │ Bun webhook receiver  │ │ Auto-Context Capture    │ │ /__annotation_context  │
             │ Claude Code channel   │ │ Console, network, vitals│ │ call + merge           │
             │ Push to session       │ │                         │ │                        │
             └──────────┬────────────┘ └────────────┬────────────┘ └───────────┬────────────┘
                        │                           │                          │
                        └───────────┬───────────────┘──────────────────────────┘
                                    │
 Phase 2     ┌──────────────────────▼──────────────────────────┐
 (2 hrs)     │ Epic 8: End-to-End Integration + Demo           │
             │ Wire channel to Go webhook                      │
             │ Test full flow: capture → store → channel → fix │
             │ Demo recording                                  │
             └─────────────────────────────────────────────────┘
```

---

## Epic 5: Channel Server

**Owner**: Session A
**Phase**: 1 (starts immediately)
**Duration**: 3–4 hours
**Dependencies**: Go server webhook dispatcher (exists), Claude Code v2.1.80+ channels API

### Context

The Go server already fires a webhook POST with the full W3C annotation envelope on every `POST /api/annotations`. The channel server receives this webhook and pushes the annotation into an active Claude Code session as a channel notification. Claude Code then uses the existing MCP tools (`list_annotations`, `get_annotation_image`, `resolve_annotation`) to read and act on the annotation.

### Tasks

#### 5.1 — Bun project scaffold

```
channel/
├── src/
│   ├── index.ts          # entry point
│   ├── server.ts         # HTTP server (webhook receiver)
│   └── channel.ts        # Claude Code channel integration
├── package.json
├── tsconfig.json
└── CLAUDE.md
```

Initialize with `bun init`. Add CLAUDE.md explaining the channel server's role and how to run it.

#### 5.2 — Webhook receiver endpoint

HTTP server listening on configurable port (default 8091, env `CHANNEL_PORT`).

```
POST /webhook/annotation
```

Accepts the W3C annotation envelope from the Go server's webhook dispatcher. Validates the payload shape (must have `id`, `type: "Annotation"`, `body`, `target`). Returns 200 immediately — processing is async.

#### 5.3 — Claude Code channel capability

Implement the `claude/channel` capability. The channel server:

1. Registers itself with Claude Code on startup
2. On receiving a webhook, formats the annotation as a channel notification
3. Pushes via `mcp.notification('notifications/claude/channel', ...)` into the active session

The notification payload should include:
- Annotation ID
- Comment text (extracted from `body[].TextualBody` where `purpose: "commenting"`)
- Page URL (`target.source`)
- Viewport (`target.state.value`)
- CSS selector (if present)
- Image URL (constructed from annotation ID)
- Domain, worktree, branch (from denormalized fields)

This gives Claude enough context to decide whether to act immediately or queue the annotation.

#### 5.4 — Reply tool for resolving back

Implement a tool that Claude Code can call through the channel to resolve an annotation back. This calls the Go server's `POST /api/annotations/:id/resolve` endpoint.

#### 5.5 — Instructions for Claude

The channel server includes instructions text that gets pushed to the Claude Code session, telling it:
- What annotations are and how to interpret them
- To use `list_annotations` MCP tool to get full details
- To use `get_annotation_image` to view the screenshot
- To attempt a fix if the annotation describes a code issue in the current worktree
- To use `resolve_annotation` with commit hash / PR link after fixing
- To explain what it did if the fix is non-trivial

#### 5.6 — Docker Compose integration

Add the channel server to `docker-compose.yml` (or document the `bun run` command in justfile). Add just commands:
- `just channel` — `cd channel && bun run src/index.ts`

### Completion criteria

- [ ] `just channel` starts the server on port 8091
- [ ] Go server webhook delivers to channel server (set `WEBHOOK_URL=http://localhost:8091/webhook/annotation`)
- [ ] Channel server logs received annotations
- [ ] Channel notification format is correct for Claude Code channels API
- [ ] Resolve-back tool calls Go server successfully

### Open questions

- Claude Code channels are research preview (v2.1.80+) — verify the exact notification API shape. Check Claude Code docs or source if available.
- Does the channel need to maintain a persistent connection, or is it push-per-event?

---

## Epic 6: Auto-Context Capture

**Owner**: Session B
**Phase**: 1 (starts immediately, no dependencies)
**Duration**: 2–3 hours

### Context

Every annotation should automatically capture technical context so Claude Code can understand the environment without the developer describing it manually. This context goes into the W3C annotation as `body[]` entries with `purpose: "describing"`.

### Tasks

#### 6.1 — Console error capture

Content script listens for console errors and warnings. On annotation capture, include the last N entries (configurable, default 10) captured since page load.

```js
// Monkey-patch console.error and console.warn before page scripts run
// Store entries in a ring buffer
// On capture, serialize as W3C body:
{
  "type": "TextualBody",
  "value": "TypeError: Cannot read properties of undefined (reading 'map')\n  at Dashboard.tsx:42",
  "purpose": "describing",
  "format": "text/plain",
  "x:role": "console-errors"
}
```

Must run at `document_start` (update manifest) to catch errors from page initialization.

#### 6.2 — Failed network request capture

Content script intercepts fetch and XMLHttpRequest to track failed requests (4xx/5xx status codes). On annotation capture, include failures since page load.

```js
{
  "type": "TextualBody",
  "value": "GET /api/users 500 Internal Server Error\nPOST /api/login 422 Unprocessable Entity",
  "purpose": "describing",
  "format": "text/plain",
  "x:role": "network-errors"
}
```

#### 6.3 — Core web vitals snapshot

Capture LCP, FID/INP, CLS at the moment of annotation using `PerformanceObserver` or the `web-vitals` library (lightweight, can vendor).

```js
{
  "type": "TextualBody",
  "value": "LCP=1.2s CLS=0.05 INP=120ms",
  "purpose": "describing",
  "format": "text/plain",
  "x:role": "web-vitals"
}
```

#### 6.4 — User agent and page metadata

Capture user agent string and basic page metadata (title, meta description) alongside existing viewport capture.

#### 6.5 — Wire into annotation assembly

The content script's W3C annotation assembly (content.js ~line 719) currently builds `body[]` with the comment and image. Extend it to append describing bodies from the context capture modules.

### Completion criteria

- [ ] Console errors appear in annotation body (verify via API response)
- [ ] Failed network requests appear in annotation body
- [ ] Web vitals snapshot captured
- [ ] Context bodies use W3C `purpose: "describing"` with `x:role` discriminator
- [ ] No performance impact on normal browsing (passive listeners, ring buffer)

---

## Epic 7: Hook System in Extension

**Owner**: Session C
**Phase**: 1 (starts immediately, no dependencies)
**Duration**: 1–2 hours

### Context

The plugin already ships a `setup-hooks` skill that generates `/__annotation_context` middleware for Next.js, Vite, and Phoenix. The extension needs to call this endpoint on capture and merge the response into the annotation.

### Tasks

#### 7.1 — Call `/__annotation_context` on capture

When the user initiates a capture, the content script (or background worker) fetches `/__annotation_context` on the current page's origin:

```js
const url = new URL('/__annotation_context', window.location.origin)
const controller = new AbortController()
setTimeout(() => controller.abort(), 500) // 500ms timeout

try {
  const res = await fetch(url, { signal: controller.signal })
  if (res.ok) {
    const context = await res.json()
    // merge into annotation
  }
} catch {
  // graceful degradation — proceed without enrichment
}
```

#### 7.2 — Merge context into annotation

Known fields from the hook response get mapped to the annotation:
- `worktree` → denormalized `worktree` column
- `branch` → denormalized `branch` column
- `commit` → stored in annotation extras
- `port` → stored in annotation extras

Everything else goes into a `TextualBody` with `purpose: "describing"` and `x:role: "hook-context"`.

#### 7.3 — Verify with setup-hooks skill output

Test against a local dev server that has the `/__annotation_context` middleware installed (use one of the templates from `plugin/skills/setup-hooks/templates/`). Verify the context appears in the stored annotation.

### Completion criteria

- [ ] Extension calls `/__annotation_context` on capture
- [ ] 500ms timeout, graceful degradation on 404/timeout
- [ ] Known fields (worktree, branch) populate denormalized columns
- [ ] Unknown fields stored in annotation body
- [ ] Works correctly when endpoint doesn't exist (no error shown to user)

---

## Epic 8: End-to-End Integration & Demo

**Owner**: All sessions converge
**Phase**: 2 (starts when Epics 5, 6, 7 are complete — or Epic 5 at minimum)
**Duration**: 1–2 hours
**Dependencies**: Epic 5 (channel server), Epic 6 (auto-context), Epic 7 (hooks)

### Tasks

#### 8.1 — Wire Go server webhook to channel server

Set `WEBHOOK_URL=http://localhost:8091/webhook/annotation` in `.env`. Verify: create annotation via extension → Go server stores it → webhook fires → channel server receives it → channel pushes to Claude Code session.

#### 8.2 — Test MCP tool flow from Claude Code

In a Claude Code session with the plugin installed:
1. Receive channel notification about a new annotation
2. Call `list_annotations` to get full details
3. Call `get_annotation_image` to view the screenshot
4. Read the auto-captured context (console errors, network failures, vitals)
5. Locate the relevant source code
6. Make a fix
7. Call `resolve_annotation` with commit hash

#### 8.3 — End-to-end integration test script

Extend `test-e2e.sh` or create `test-e2e-channel.sh`:
1. Start Go server + channel server
2. Create annotation via API (simulating extension)
3. Verify webhook delivered to channel server
4. Verify channel notification format
5. Resolve annotation via MCP
6. Verify annotation state is `resolved`

#### 8.4 — Demo scenario prep

Prepare a demo project (e.g., a simple Next.js or Phoenix app with a deliberate UI bug). Script the demo flow:
1. Open the app in Chrome
2. Ctrl+Shift+A → select the buggy area → draw arrow → add comment
3. Show the annotation in the side panel (with auto-captured console error)
4. Switch to terminal — show Claude Code receiving the annotation in real-time
5. Claude reads the screenshot, identifies the issue, fixes it
6. Annotation auto-resolved with commit hash
7. Refresh browser — bug is gone

#### 8.5 — Demo recording

Record the demo for LinkedIn. Keep it under 90 seconds. Focus on the magic moment: human observes → agent acts.

### Completion criteria

- [ ] Full flow works: capture → store → channel → Claude Code → fix → resolve
- [ ] Under 60 seconds end-to-end (success criteria from ROADMAP)
- [ ] Demo recorded

---

## Session Assignment Summary

| Session | Phase 1 (morning, 3-4 hrs) | Phase 2 (afternoon, 2 hrs) |
|---------|----------------------------|----------------------------|
| **A** | Epic 5: Channel server (Bun) | Epic 8: Integration lead |
| **B** | Epic 6: Auto-context capture | Epic 8: Assist + polish |
| **C** | Epic 7: Hook system in extension | Epic 8: Demo prep + recording |

## Synchronization Points

One sync point: **End of Phase 1**. Epic 5 (channel server) must work before Phase 2 integration begins. Epics 6 and 7 are nice-to-have for the demo — the demo works without them (just with less rich context).

If Epic 5 runs long, Sessions B and C should assist after finishing their epics.

## Sunday Exit Criteria

- [ ] Channel server receives webhooks and pushes to Claude Code
- [ ] Auto-context capture enriches annotations (console, network, vitals)
- [ ] Hook system calls `/__annotation_context` with graceful degradation
- [ ] End-to-end flow demonstrated: capture → store → channel → Claude Code → fix
- [ ] Demo recorded for LinkedIn
- [ ] Responsive viewports deferred (documented in ROADMAP as post-hackathon)

## Risk: Claude Code Channels API

Channels are research preview (v2.1.80+). The exact API shape may differ from what's documented. Session A should spend the first 30 minutes spiking the channel integration before committing to the full architecture. If the API is unstable or unavailable, fallback plan: polling-based integration where Claude Code periodically calls `list_annotations` with a `since` filter instead of receiving push notifications.
