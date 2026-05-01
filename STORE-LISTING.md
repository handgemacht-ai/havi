# Chrome Web Store Listing — Reference

This document is a reference for the human filling out the Chrome Web Store Developer Dashboard listing form. It is **not** consumed by CI. Update it whenever the listing changes.

## Item details

- **Name:** Annotation Platform
- **Category:** Developer Tools
- **Language:** English
- **Short description** (max 132 chars):
  > Capture visual annotations on any web page and ship them to your self-hosted annotation server. W3C Web Annotation format.

## Detailed description

> **Annotation Platform** is a self-hosted visual annotation tool for developer teams. Capture screenshots, draw markup on top of any page, attach a comment, and send the result to your own annotation server.
>
> **Key features**
> - **One-shortcut capture.** Press `Ctrl+Shift+A` (or `Cmd+Shift+A` on macOS) on any web page to start annotating.
> - **Drawing tools.** Rectangles, arrows, freeform highlights, and text labels powered by Fabric.js.
> - **Region selection.** Pick a precise area of the page to capture, with resize handles.
> - **W3C Web Annotation format.** Annotations are stored as W3C Web Annotations, including CSS selectors and fragment selectors so the targeted element can be resolved later.
> - **Self-hosted.** Annotations are sent to a server you run yourself — defaults to `http://localhost:8090`. No data goes to a third party.
> - **Optional remote server.** If you run a shared annotation server for your team, you can grant the extension host permission for it from the side panel.
> - **Side panel browser.** List, filter, and manage all your annotations directly inside Chrome.
> - **Claude Code integration.** Annotations created with this extension can be picked up in real time by Claude Code sessions via the bundled MCP channel server, so a coding agent can act on visual feedback as you capture it.
>
> **Privacy:** This extension does not send data to the publisher. All annotation data goes to the server URL you configure. No analytics, no telemetry, no ads. See [PRIVACY.md](https://github.com/handgemacht-ai/havi/blob/main/PRIVACY.md).
>
> **Source code:** https://github.com/handgemacht-ai/havi

## Permission justifications

The CWS form asks for a justification for every permission. Use the table below — keep it in sync with `extension/manifest.json`.

| Permission | Justification |
|---|---|
| `activeTab` | Required to capture a screenshot of the user's current tab via `chrome.tabs.captureVisibleTab` when the user triggers an annotation. |
| `scripting` | Required to inject the annotation overlay (drawing tools, region selector) into the active page. |
| `storage` | Required to persist user preferences such as the annotation server URL and capture defaults via `chrome.storage.sync`. |
| `sidePanel` | Required to render the annotation list and management UI in Chrome's side panel. |
| Content scripts matching `<all_urls>` | Annotation capture must work on whichever page the user is currently viewing; the script only runs the capture overlay logic and does not exfiltrate page content. |
| `host_permissions` for `http://localhost:*/` and `http://127.0.0.1:*/` | Required to send annotations to the user's local annotation server (the default deployment). |
| `optional_host_permissions` for `https://*/*` and `http://*/*` | Used only if the user opts in to sending annotations to a self-hosted remote server. The extension does not access these hosts unless the user grants the permission. |

## Single-purpose statement

> The single purpose of this extension is to capture screenshot-based annotations of web pages and send them to a user-configured annotation server.

## Required assets

The dashboard requires these assets at submission. Production-ready files live in the listed paths or must be produced manually before upload.

- [x] **Icon 128×128** — `extension/assets/icon-128.png` (referenced by `manifest.json`)
- [ ] **Screenshots** — at least one, 1280×800 or 640×400 PNG/JPEG. Capture manually:
  - Capture overlay in action on a real web page
  - Side panel listing several annotations
  - Annotation detail view
- [ ] **Promo tile (small)** — optional, 440×280 PNG/JPEG
- [ ] **Promo tile (marquee)** — optional, 1400×560 PNG/JPEG (only used if the listing is featured)

## Privacy practices

The CWS form has a separate "Privacy practices" tab. Fill it in as follows:

- **Single purpose:** see above.
- **Permissions justifications:** copy from the table above.
- **Data collection disclosure:**
  - Personally identifiable information: **No**
  - Health information: **No**
  - Financial and payment information: **No**
  - Authentication information: **No**
  - Personal communications: **No**
  - Location: **No**
  - Web history: **No**
  - User activity: **Yes** — the user's own annotation actions are stored, but only on a server the user controls.
  - Website content: **Yes** — screenshots and CSS selectors of pages the user actively annotates, stored only on the user's own server.
- **Privacy policy URL:** `https://github.com/handgemacht-ai/havi/blob/main/PRIVACY.md`
- **"I certify that…" checkboxes:** all four (data not sold, not used for unrelated purposes, not used to determine creditworthiness, complies with the Developer Program Policies).

## One-time human setup checklist

Before the GitHub Actions publish workflow can run, a human must complete each of these once:

- [ ] Create or choose a Chrome Web Store **developer account** ($5 one-time fee).
- [ ] In the dashboard, create the listing and do the **first manual zip upload** (the workflow can only update an existing item — it cannot create one).
- [ ] Complete the listing form using this document, upload the icon and screenshots, and submit for review (or save as draft if you want to gate publishing on the workflow).
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
