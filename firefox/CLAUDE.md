# Firefox Port

Manifest V3 Firefox port of the HAVI extension. The Chrome extension at `../extension/` is the source of truth — this directory contains only Firefox-specific overlay files (currently just the manifest).

## Differences from Chrome

| Concern | Chrome | Firefox |
|---------|--------|---------|
| Side panel | `side_panel.default_path` + `sidePanel` permission | `sidebar_action.default_panel` |
| Open-on-action | `chrome.sidePanel.setPanelBehavior({ openPanelOnActionClick: true })` | `chrome.action.onClicked` → `browser.sidebarAction.toggle()` (feature-detected in shared background.js) |
| Background | `service_worker` | `scripts` (event page) |
| Addon id | implicit | `browser_specific_settings.gecko.id = "havi@handgemacht.ai"` |
| Capture shortcut | `Ctrl/Cmd+Shift+A` | `Alt+Shift+A` (`Ctrl+Shift+A` is reserved by Firefox for Add-ons Manager) |

## Building

```bash
just build-firefox       # produces firefox-build/ at the repo root
```

The build script (`../scripts/build-firefox.sh`) rsyncs `../extension/` into `firefox-build/` then overlays everything in this directory.

## Loading unpacked in Firefox

1. Run `just build-firefox`
2. Open `about:debugging#/runtime/this-firefox`
3. Click "Load Temporary Add-on…"
4. Select `firefox-build/manifest.json`

## Sidebar UX

Firefox does not auto-open a sidebar when the toolbar action is clicked. The shared `background.js` wires `chrome.action.onClicked` → `browser.sidebarAction.toggle()` so the toolbar button mirrors Chrome's side panel toggle.
