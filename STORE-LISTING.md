# Chrome Web Store Listing — HAVI

Paste-ready copy for the Chrome Web Store Developer Dashboard. Update this doc whenever the listing changes — it is the source of truth, not the dashboard.

---

## Item details

### Name (manifest-driven, max 75 chars)
```
HAVI – Human-Agent Visual Interface
```

### Category
`Developer Tools`

### Language
`English (United States)`

### Short description (max 132 chars)
```
Visual annotations on any web page, sent to your self-hosted server. Bridge human feedback to AI coding agents in real time.
```
*(125 chars)*

---

## Detailed description (paste into the 16,000-char field)

```
HAVI — Human-Agent Visual Interface

The bridge between human visual feedback and AI coding agents.

If you've ever shipped a screenshot to your AI agent in Slack and watched it guess at what you meant, HAVI is for you. Capture annotations directly on any web page, attach a comment, and your local AI coding agent sees them the moment you save — with the screenshot, the targeted DOM element, the page URL, and the viewport already attached.

═══════════════════════════════════════
WHAT IT DOES
═══════════════════════════════════════

• One-shortcut capture — press Ctrl+Shift+A (or ⌘+Shift+A on macOS) on any page and start annotating.
• Drawing tools — rectangles, arrows, freeform highlights, and text labels powered by Fabric.js.
• Region selection — pick a precise area of the page with resizable handles, not just "the whole tab".
• Element-aware — every annotation stores a CSS selector for the targeted element, so an agent or test can resolve it later.
• Side panel browser — list, filter, and manage your annotations directly inside Chrome.
• Self-hosted — annotations go to a server you run, not to a third party. No accounts, no telemetry.
• W3C Web Annotation format — your annotations are portable, standards-based JSON, not a proprietary blob.

═══════════════════════════════════════
THE AGENT BRIDGE (the H–A in HAVI)
═══════════════════════════════════════

HAVI ships with a companion MCP server (Model Context Protocol). When your local Claude Code session is running, every annotation you create lands in the agent's context within seconds — including the screenshot, the comment, the URL, the viewport, and any console errors collected from the page.

Capture a misaligned button on your dashboard. Type "fix this and add a regression test." Switch to your terminal — the agent already has it.

═══════════════════════════════════════
WHO IT'S FOR
═══════════════════════════════════════

• Developers building web apps with AI coding agents (Claude Code, etc.)
• Small teams that want a private, self-hosted alternative to commercial visual feedback tools
• Anyone tired of pasting screenshots into chat and re-explaining where the problem is

═══════════════════════════════════════
PRIVACY
═══════════════════════════════════════

• No data goes to the publisher. All annotations go to a server URL you configure (defaults to http://localhost:8090).
• No analytics, no tracking, no telemetry, no advertising.
• No background page monitoring — captures happen only when you actively trigger them.
• Optional remote server is opt-in: you grant host permission for one specific URL, revocable from chrome://extensions.
• Full privacy policy: https://github.com/handgemacht-ai/havi/blob/main/PRIVACY.md

═══════════════════════════════════════
SETUP (about 5 minutes)
═══════════════════════════════════════

1. Install the extension.
2. Run the open-source HAVI server on your own machine (a single binary or `just up && just server`).
3. Open the side panel, confirm the server URL.
4. Press Ctrl+Shift+A on any page and capture your first annotation.
5. Optional: hook the bundled MCP server into your Claude Code config for the agent bridge.

Server source code, installation guide, and documentation:
https://github.com/handgemacht-ai/havi

═══════════════════════════════════════
OPEN SOURCE
═══════════════════════════════════════

HAVI is fully open source. Read the code, run your own server, fork it, contribute back.
Repository: https://github.com/handgemacht-ai/havi
Issues:     https://github.com/handgemacht-ai/havi/issues
License:    see repository

Built by handgemacht.ai.
```

*(approx. 2,400 chars — well under the 16,000 cap, leaves room to grow)*

---

## URLs

| Field | Value |
|---|---|
| Homepage URL | `https://github.com/handgemacht-ai/havi` |
| Support URL | `https://github.com/handgemacht-ai/havi/issues` |
| Official URL | leave `None` unless you've verified domain ownership in Search Console for `handgemacht.ai` |
| Mature content | `No — does not contain mature content` |

---

## Privacy practices tab

- **Single purpose:**
  > Capture screenshot-based visual annotations of web pages and send them to a user-configured annotation server.

- **Permission justifications** — paste these one-by-one when the form asks per permission:

| Permission | Justification |
|---|---|
| `activeTab` | Captures a screenshot of the user's current tab via `chrome.tabs.captureVisibleTab` when the user explicitly triggers an annotation via the keyboard shortcut or toolbar action. |
| `scripting` | Injects the annotation overlay (drawing tools and region selector) into the active page so the user can mark up the screenshot region. |
| `storage` | Persists user preferences such as the annotation server URL and capture defaults via `chrome.storage.sync`. |
| `sidePanel` | Renders the annotation list and management UI in Chrome's side panel so users can browse, filter, and delete their annotations. |
| Content scripts on `<all_urls>` | Annotation capture must work on whichever page the user is currently viewing. The script only runs the capture overlay logic and does not exfiltrate page content. |
| `host_permissions` for `http://localhost/*` and `http://127.0.0.1/*` | Required to send annotations to the user's local annotation server (the default deployment). Chrome match patterns ignore the port, so these cover the configurable port (default 8090). |
| `optional_host_permissions` for `https://*/*` and `http://*/*` | Used only if the user explicitly opts in to sending annotations to a self-hosted remote server instead of localhost. Permission is requested at runtime and revocable. |

- **Data collection disclosure:**
  - Personally identifiable information: **No**
  - Health information: **No**
  - Financial and payment information: **No**
  - Authentication information: **No**
  - Personal communications: **No**
  - Location: **No**
  - Web history: **No**
  - User activity: **Yes** — the user's own annotation actions are stored, but only on a server the user controls. None of this data leaves the user's infrastructure.
  - Website content: **Yes** — screenshots and CSS selectors of pages the user actively annotates, stored only on the user's own server.

- **Privacy policy URL:** `https://github.com/handgemacht-ai/havi/blob/main/PRIVACY.md`

- **"I certify that…" checkboxes:** all four (data not sold to third parties, not used for unrelated purposes, not used for credit decisions, complies with the Developer Program Policies).

---

## Graphic assets

| Asset | Spec | Status |
|---|---|---|
| Store icon | 128×128 PNG | ✅ `extension/assets/icon-128.png` |
| Screenshot 1 | 1280×800 or 640×400 | ❌ manual: capture overlay in action |
| Screenshot 2 | 1280×800 or 640×400 | ❌ manual: side panel listing annotations |
| Screenshot 3 | 1280×800 or 640×400 | ❌ manual: agent picking up an annotation in Claude Code |
| Small promo tile | 440×280 | optional |
| Marquee promo tile | 1400×560 | optional, only used if listing is featured |
| Promo video | YouTube URL | optional |

Screenshot suggestions (the more concrete the better):
1. **The capture flow:** real web app (e.g. a dashboard) with the HAVI overlay open, a region selected, a comment typed, an arrow drawn — caption "Capture annotations on any page".
2. **The side panel:** Chrome side panel showing 4–5 annotations across different pages, each with thumbnail + comment + timestamp — caption "Browse, filter, manage".
3. **The agent bridge:** split view showing the side panel on one side and a Claude Code terminal session on the other where the agent has just received the annotation — caption "Your agent sees it the moment you save".

---

## One-time human setup checklist

Before the GitHub Actions publish workflow can run, a human must complete each of these once:

- [ ] Create or choose a Chrome Web Store **developer account** ($5 one-time fee).
- [ ] In the dashboard, create the listing and do the **first manual zip upload** (the workflow can only update an existing item — it cannot create one).
- [ ] Complete the listing form using this document, upload the icon and screenshots, and submit for review.
- [ ] Note the **Item ID** shown in the dashboard URL (`https://chrome.google.com/webstore/devconsole/.../item/<ITEM_ID>`).
- [ ] Create a **GCP project** with the **Chrome Web Store API** enabled.
- [ ] Create an **OAuth 2.0 Client ID** of type "Desktop app" — record the client ID and client secret.
- [ ] Generate a **refresh token** for that client (one-time OAuth flow with scope `https://www.googleapis.com/auth/chromewebstore`).
- [ ] Add four secrets to the GitHub repository (`Settings → Secrets and variables → Actions`):
  - [ ] `CHROME_EXTENSION_ID` (the Item ID)
  - [ ] `CHROME_CLIENT_ID`
  - [ ] `CHROME_CLIENT_SECRET`
  - [ ] `CHROME_REFRESH_TOKEN`

After this checklist is complete, releases happen via `just release <version>` and the workflow takes over.
