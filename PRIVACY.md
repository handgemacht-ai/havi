# Privacy Policy — HAVI

**Last updated:** 2026-05-01

This Chrome extension ("HAVI") is published by handgemacht.ai for use by developer teams who want to capture visual annotations against the web pages they are working on. This document describes what the extension collects, where that data goes, and what it does not do.

## What the extension collects

When you actively use the extension (by pressing the keyboard shortcut, clicking the toolbar action, or using the side panel), it may collect:

- **Screenshots** of the visible browser tab, captured via `chrome.tabs.captureVisibleTab`
- **Annotation text** that you type into the capture overlay or side panel
- **CSS selectors** and DOM coordinates of the element or region you target
- **Page URL** of the tab the annotation was captured on
- **User preferences** (annotation server URL, capture defaults) stored in `chrome.storage.sync`

The extension does **not** collect anything when you are not actively annotating. It does not track your browsing history, monitor pages in the background, or read page content beyond what is required to capture the screenshot and resolve the selector for the region you target.

## Where your data goes

All annotations are sent to an annotation server **that you configure**. There is no first-party server operated by the extension publisher.

- **Default mode (localhost):** the extension sends annotations to `http://localhost:8090` — a server running on your own machine.
- **Optional remote mode:** you may grant the extension host permission for a specific server (for example, a self-hosted instance on your team's infrastructure). The extension will then send annotations to that URL instead. Granting that permission is opt-in and revocable from `chrome://extensions`.

Annotations are stored on your server using the [W3C Web Annotation](https://www.w3.org/TR/annotation-model/) data model. The publisher of this extension does not receive, see, or store any of your annotation data.

## What the extension does not do

- **No third-party sharing.** Annotation data is sent only to the server URL you configure. It is not transmitted to the extension publisher or any third party.
- **No analytics or telemetry.** The extension does not send usage metrics, error reports, or any other data to the publisher.
- **No advertising.** The extension does not contain ad SDKs and does not interact with ad networks.
- **No background monitoring.** The extension does not capture screenshots or read page content unless you have actively triggered a capture.
- **No account system.** The extension does not require, create, or transmit user accounts or credentials.

## Permissions used and why

| Permission | Purpose |
|---|---|
| `activeTab` | Capture the visible tab when you trigger an annotation. |
| `scripting` | Inject the annotation overlay into the active page so you can draw and select regions. |
| `storage` | Save your preferences (annotation server URL, capture defaults). |
| `sidePanel` | Display the side panel UI for browsing and managing annotations. |
| Content scripts on `<all_urls>` | Annotation capture must work on whatever page you are currently looking at; the script only runs the capture overlay logic. |
| Optional host permissions | Granted only if you choose to send annotations to a remote server instead of `localhost`. |

## Your control over your data

- You can delete any annotation at any time via the side panel or directly via the annotation server's API (`DELETE /api/annotations/:id`).
- You can clear all stored preferences by removing the extension from `chrome://extensions`.
- You can revoke optional host permissions at any time from `chrome://extensions`.
- Because your annotation server is under your control, you can also delete the entire database whenever you wish.

## Contact

Questions about this policy or the extension's data handling can be sent to the project maintainers at https://github.com/handgemacht-ai/havi/issues.
