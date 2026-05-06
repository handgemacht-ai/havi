# Sidebar (Side Panel) — Feature Spec

The Chrome extension's side panel (`extension/src/sidepanel/`) is the developer cockpit for capturing, triaging, and resolving annotations alongside Claude Code. This document is both an inventory of what is shipped today and an ambitious spec for where the surface is going.

Status legend:

- **Shipped** — implemented in `extension/src/sidepanel/`.
- **Planned** — explicit in [ROADMAP.md](ROADMAP.md) but not yet rendered in the sidebar.
- **Aspirational** — not on the roadmap; flagged as the natural next move once foundations land.

---

## 1. Header & Identity

### 1.1 Title and connection status — *Shipped*
- "Annotations" wordmark plus a status dot tied to the periodic `check-health` ping.

### 1.2 Settings toggle — *Shipped*
- Gear icon expands the settings panel.

### 1.3 Channel/session indicator — *Aspirational*
- Surface whether a Claude Code session is currently attached to the channel server. Three states: **no session**, **session attached**, **session attached + watching this domain**.
- Hover/tap reveals the session id, working directory, and worktree path so the user knows *which* agent will receive their next annotation.

### 1.4 Identity badge — *Aspirational*
- Avatar + display name of the current creator, sourced from the identity store (see §2.2).
- Click → "switch identity" / "sign out" once SSO lands.

---

## 2. Settings Panel

### 2.1 Server URL — *Shipped*
- URL input with `http(s)://` validation.
- Save persists via the background worker; non-localhost hosts trigger a `chrome.permissions.request` for the new origin.
- Inline status line (`success` auto-clears after 2s).

### 2.2 Identity / display name — *Planned*
- Free-text "creator" name written into every annotation's `creator.name`.
- Foundation for the author filter and multi-user views before SSO is wired.

### 2.3 Google SSO — *Planned*
- Sign-in flow that replaces the free-text identity once the Go server has the auth interface.
- Token stored via `chrome.storage.session`; signed-out state hides creator-scoped views.

### 2.4 Project & worktree binding — *Aspirational*
- Lets the developer pin the panel to a project (not just a domain) so annotations from `localhost:4000` and `feature-x.dev.handgemacht.ai` show up in the same list.
- Auto-detected from the Claude Code plugin's environment script; manual override available.

### 2.5 Capture defaults — *Aspirational*
- Default motivation (`commenting`, `highlighting`, `describing`).
- Toggle for "always include console + network snapshot".
- Default viewport preset for responsive captures.

### 2.6 Notifications — *Aspirational*
- Toggle for desktop notifications when an agent resolves an annotation you opened.

### 2.7 Theme — *Aspirational*
- Light / dark / system. Today the panel follows a fixed scheme.

---

## 3. Status & Diagnostics

### 3.1 Server health banner — *Shipped*
- "Server unavailable" banner driven by `setInterval(checkHealth, 30s)`.

### 3.2 Channel health — *Aspirational*
- Distinct indicator for "Go server up but no Claude Code channel listening" — today both failures collapse into the same dot.

### 3.3 Diagnostics panel — *Aspirational*
- One-tap "copy diagnostics" producing server URL, last health timestamp, last 10 channel events, and Chrome version. Cuts the loop when something breaks during a hackathon demo.

---

## 4. Filter Bar

### 4.1 State chip filters — *Shipped*
- `All` / `Open` / `Resolved`, mutually exclusive, one always active.

### 4.2 Viewport filter — *Planned*
- Chip group for `mobile`, `tablet`, `desktop`, plus a "custom" entry derived from the recorded `viewport=WxH` state value.

### 4.3 Worktree filter — *Planned*
- Multi-select of worktree paths seen in the current result set, sourced from the indexed `worktree` column.

### 4.4 Branch filter — *Planned*
- Multi-select of git branches, hooks-supplied. Useful when several branches share a domain.

### 4.5 Author filter — *Planned*
- Multi-select of `creator.name` values; "mine" pin for the current identity.

### 4.6 Motivation filter — *Aspirational*
- Toggle between `commenting` (human notes), `highlighting` (visual emphasis only), `describing` (auto-context).

### 4.7 Time range filter — *Aspirational*
- Quick chips: `today`, `last 7 days`, `since branch start`, `custom`.

### 4.8 Free-text search — *Aspirational*
- Debounced search across comment + element text. Server-side once full-text indexing exists; client-side fallback in the meantime.

### 4.9 Sort — *Aspirational*
- Toggle between `newest`, `oldest`, `most recently modified`, `unresolved first`.

### 4.10 Saved filter views — *Aspirational*
- Persist a filter combination as a named view (e.g., "P1 mobile bugs on `feature-x`").

---

## 5. Domain & Project Scope

### 5.1 Auto-scope to current tab domain — *Shipped*
- List always filtered to `new URL(activeTab.url).host`.

### 5.2 Domain switcher — *Planned*
- Dropdown of recently captured domains; "all domains in this project" entry.

### 5.3 Project switcher — *Aspirational*
- Sit above the domain switcher; lets a single panel show annotations across `havi`, `spotter`, etc., when the developer is reviewing a multi-rig change.

### 5.4 Cross-worktree comparison — *Aspirational*
- Side-by-side list of annotations on the same URL across two worktrees — the "did the rebase fix it?" view.

---

## 6. Capture Bar

### 6.1 Capture region — *Shipped*
- Drag-to-select with resize handles via Cropper.js. Cancel state when active.

### 6.2 Pick element — *Shipped*
- Element picker via `css-selector-generator`. Cancel state when active.

### 6.3 Mutual exclusion — *Shipped*
- Only one capture mode runs at a time; `capture-ended` resets both buttons.

### 6.4 Keyboard shortcut hint — *Shipped*
- `Ctrl+Shift+A` advertised in the bar.

### 6.5 Full-page / scrolling capture — *Aspirational*
- Stitch the visible viewport into a full-page screenshot. Roadmap explicitly excludes this from v1; pulling it in is a deliberate scope expansion.

### 6.6 Annotate-from-image — *Aspirational*
- Drop a screenshot from outside the browser (e.g. mobile device, Figma export) into the panel and annotate it as if captured live.

### 6.7 Quick capture (no markup) — *Aspirational*
- One-click "snap and save" that skips the drawing canvas — useful for triaging during a long QA session.

---

## 7. Capture Alert

### 7.1 Permission grant retry — *Shipped*
- Detects `permission_required`, offers "Grant access" via `chrome.permissions.request({ origins: ['<all_urls>'] })`, retries automatically on success.

### 7.2 Unsupported page — *Shipped*
- Friendly message for `chrome://`, store, and other restricted pages.

### 7.3 Generic failure — *Shipped*
- Surfaces the underlying error from the background worker.

### 7.4 Server unreachable — *Aspirational*
- Distinct alert (vs. the global banner) that points at the settings panel and pre-flags the URL field.

---

## 8. Annotation List — Card Summary

### 8.1 Thumbnail — *Shipped*
- Lazy-loaded `GET /api/annotations/:id/image` with placeholder fallback.

### 8.2 Comment preview — *Shipped*
- First 80 characters of the user comment.

### 8.3 State / viewport / time chips — *Shipped*
- Visible in the summary; state chip is colour-coded.

### 8.4 Worktree, branch, author chips — *Planned*
- Add chips for `worktree` and `branch` (when set), and `creator` (always).

### 8.5 Motivation chip — *Aspirational*
- Tiny icon distinguishing user comment vs auto-context vs highlight.

### 8.6 Console / network / vitals indicators — *Aspirational*
- Inline icons on the card showing `🔴 console`, `🌐 network`, `📊 vitals` so the user can see at a glance which annotations have rich auto-context attached. Counts shown when > 0.

### 8.7 Drawn markup overlay on thumbnail — *Aspirational*
- Render the `SvgSelector` over the thumbnail so the developer's own arrows/boxes survive into the list view.

### 8.8 Resolution badge — *Aspirational*
- When state = `resolved`, show the resolver (agent vs human), plus a tiny chip of the resolve metadata: `bead`, `commit`, or `pr` (whichever the agent supplied).

### 8.9 Single-card expand — *Shipped*
- One open detail at a time; chevron rotates when expanded.

### 8.10 Multi-select mode — *Aspirational*
- Long-press / shift-click to enter selection mode for bulk actions.

---

## 9. Empty States

### 9.1 No annotations — *Shipped*
- Icon + `Press Ctrl+Shift+A to capture your first annotation.`

### 9.2 Filtered empty — *Shipped*
- "No `<state>` annotations." Aspirational extension: include the active filters in the message and a one-tap "Clear filters" button.

### 9.3 Server error — *Shipped*
- "Cannot connect to server. Check settings." Aspirational extension: link directly to the settings panel.

### 9.4 First-run onboarding — *Aspirational*
- Three-step coachmarks: capture → markup → see it in Claude Code. Reset via a settings entry.

---

## 10. Annotation Detail (expanded card)

### 10.1 Full screenshot — *Shipped*
- Full-resolution image; hidden if it 404s.

### 10.2 Drawn markup overlay — *Aspirational*
- Composite SVG from `SvgSelector` rendered over the screenshot; toggle to hide markup for a clean view.

### 10.3 Comment block — *Shipped*
- Full text of the `commenting` body.

### 10.4 Element text + CSS selector — *Shipped*
- First 200 chars of `purpose=describing` body, plus the captured CSS selector.

### 10.5 Structured "Console" pane — *Planned*
- Dedicated section listing console errors and warnings captured at the moment of annotation. Each entry shows level, message, source location.

### 10.6 Structured "Network" pane — *Planned*
- Failed requests (4xx/5xx) with method, URL, status, duration.

### 10.7 Structured "Web Vitals" pane — *Planned*
- LCP, CLS, FID/INP, TTFB snapshot from the moment of annotation.

### 10.8 User agent + browser metadata — *Planned*
- UA string parsed into browser/OS chips.

### 10.9 Hook-enriched context pane — *Planned*
- Renders worktree path, git branch, git commit, dev-server port, plus arbitrary fields the project's `/__annotation_context` endpoint returned. Custom fields displayed as a key/value table.

### 10.10 Metadata DL — *Shipped*
- Domain, creator, created, viewport, state. Aspirational extensions: modified timestamp, motivation, project, branch, worktree.

### 10.11 Edit comment — *Shipped*
- Inline textarea, preserves non-comment body parts; empty saves are ignored.

### 10.12 Edit markup — *Aspirational*
- "Re-open in canvas" action that reloads the screenshot + `SvgSelector` into the content-script editor and saves a new revision.

### 10.13 Delete with inline confirm — *Shipped*

### 10.14 Resolve flow — *Planned*
- Button that calls `POST /api/annotations/:id/resolve` with a small form: `bead id`, `commit hash`, `pr link`, free-text note. Pre-fills from the active Claude Code session when available.

### 10.15 Reopen — *Aspirational*
- For resolved annotations: a "reopen" action that reverts state and records a `describing` body explaining why.

### 10.16 Resolve metadata view — *Aspirational*
- When resolved, show *who* resolved it (agent vs human), *when*, and the linked artifacts (bead, commit, PR) as clickable chips.

### 10.17 Conversation thread — *Aspirational*
- Append-only list of follow-up `commenting` bodies with author + timestamp, so a teammate or agent can leave a reply without overwriting the original.

### 10.18 Send to Claude Code — *Aspirational*
- Explicit button that pushes the annotation through the channel even if no auto-push happened (e.g., session attached after creation). Mirrors the "Send to Claude Code" deferred mode in the annotations channel.

### 10.19 Copy / share — *Aspirational*
- Copy annotation ID, copy `urn:uuid:` form, copy "Open this annotation" deep link. Useful for pasting into PR descriptions or Claude Code prompts.

### 10.20 Cross-link to tracking — *Aspirational*
- Once issue tracker integrations exist (out of weekend scope), show the linked Jira/Linear/GitHub Issue as a chip with status.

---

## 11. Bulk Actions

### 11.1 Multi-select — *Aspirational*
- Selection mode in the list (see §8.10).

### 11.2 Bulk resolve — *Aspirational*
- Apply a single resolve metadata to N annotations (e.g., one PR closes ten annotations).

### 11.3 Bulk delete — *Aspirational*
- With a confirm step.

### 11.4 Bulk export — *Aspirational*
- Export selected annotations as a single Markdown bundle (comments + screenshots) for pasting into a PR description.

---

## 12. Live Updates & Refresh

### 12.1 Real-time inserts — *Shipped*
- `annotation-created` from the background prepends new annotations (respecting the active filter).

### 12.2 Tab activation refresh — *Shipped*
- `chrome.tabs.onActivated` triggers a refetch.

### 12.3 URL change refresh — *Shipped*
- `chrome.tabs.onUpdated` (when `changeInfo.url` is set) triggers a refetch.

### 12.4 Capture-end reset — *Shipped*
- `capture-ended` restores both capture buttons.

### 12.5 Real-time updates and deletes — *Aspirational*
- `annotation-updated` / `annotation-deleted` events so collaborators see each other's edits without a manual refetch.

### 12.6 Resolution-by-agent toast — *Aspirational*
- When the channel reports an agent resolved one of *your* annotations, show a transient toast with a "view" action.

---

## 13. Keyboard & Accessibility

### 13.1 Global capture shortcut — *Shipped*
- `Ctrl+Shift+A` advertised by the panel.

### 13.2 List navigation — *Aspirational*
- `j` / `k` to move, `Enter` to expand, `e` to edit, `r` to resolve, `x` to delete, `/` to focus search, `Esc` to collapse.

### 13.3 Focus management — *Aspirational*
- Trapped focus inside the delete-confirm strip and the edit textarea; arrow-key navigation between filter chips.

### 13.4 Screen reader pass — *Aspirational*
- Live region announcing new annotations, resolution events, and connection state changes.

### 13.5 High-contrast / reduced-motion — *Aspirational*
- Respect `prefers-reduced-motion` (currently the panel uses CSS transitions on chevron + alert).

---

## 14. Pagination & Volume

### 14.1 `meta.count` display — *Aspirational*
- Show total count next to the active filter set (`Open · 23`).

### 14.2 Virtualised list — *Aspirational*
- Once result sets cross ~100 entries, switch to a virtualised list to keep scroll smooth.

### 14.3 Pagination / lazy load — *Aspirational*
- Infinite scroll backed by `limit` / `offset`.

### 14.4 Archive view — *Aspirational*
- Separate section for resolved-and-old annotations, hidden from the default list once a retention window passes.

---

## 15. Safety & Conventions

### 15.1 Safe DOM construction — *Shipped*
- Comments, selectors, and metadata flow through `createTextNode` / `setAttribute`; no `innerHTML` with user data.

### 15.2 Permission boundary respect — *Shipped*
- Non-localhost server URLs require explicit `chrome.permissions` grants before the panel will save.

### 15.3 W3C envelope fidelity — *Planned*
- Every read/write goes through helpers that respect the W3C body/target/selector structure (no flattening into custom keys). The current edit path is correct for `commenting` bodies; needs extending as describing-body sub-types grow (§10.5–10.7).

### 15.4 Per-domain privacy — *Aspirational*
- Visual "this annotation will be visible to: <list>" hint at capture time, once team membership is real.

---

## Open questions

- **Describing-body schema**: do we model console / network / vitals as one structured `describing` body per category, a single fat one with sub-fields, or as separate hook-supplied extras? Decision blocks §10.5–10.7 and the chip indicators in §8.6.
- **Identity before SSO**: is a free-text creator name (§2.2) good enough for the v1 multi-user story, or do we hold the feature for SSO?
- **Resolve form depth**: minimum viable is `bead id` + free-text note (§10.14). Pre-filling from the attached Claude Code session is the magical version — depends on the channel exposing session context back to the panel.
- **Markup re-edit revisioning**: do edits create a new annotation or mutate the existing `SvgSelector`? Affects the conversation-thread design (§10.17).
- **Cross-project scope**: is the panel a per-project tool (project switcher) or a personal inbox (all projects, filtered)? Two different mental models, both defensible.
