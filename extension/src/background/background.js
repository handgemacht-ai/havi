const DEFAULT_SERVER_URL = 'http://localhost:8090';

chrome.sidePanel.setPanelBehavior({ openPanelOnActionClick: true });

async function ensureContentScript(tabId) {
  try {
    await chrome.tabs.sendMessage(tabId, { type: 'ping' });
  } catch {
    await chrome.scripting.executeScript({
      target: { tabId },
      files: ['assets/fabric.min.js', 'assets/cropper.min.js', 'assets/css-selector-generator.min.js', 'src/content/content.js'],
    });
    await chrome.scripting.insertCSS({
      target: { tabId },
      files: ['assets/cropper.min.css', 'src/content/content.css'],
    });
    await waitForContentReady(tabId);
  }
}

async function waitForContentReady(tabId, retries = 10, delay = 50) {
  for (let i = 0; i < retries; i++) {
    try {
      await chrome.tabs.sendMessage(tabId, { type: 'ping' });
      return;
    } catch {
      await new Promise((r) => setTimeout(r, delay));
    }
  }
  throw new Error('Content script not ready');
}

async function startCaptureInTab(tabId) {
  const tab = await chrome.tabs.get(tabId);
  if (!tab.url || !/^https?:\/\//.test(tab.url)) {
    throw new Error('Cannot capture this page');
  }
  const dataUrl = await chrome.tabs.captureVisibleTab(null, { format: 'png' });
  await ensureContentScript(tabId);
  const response = await chrome.tabs.sendMessage(tabId, { type: 'start-capture', dataUrl });
  if (!response?.ok) throw new Error(response?.error || 'Content script busy');
}

async function startPickElementInTab(tabId) {
  const tab = await chrome.tabs.get(tabId);
  if (!tab.url || !/^https?:\/\//.test(tab.url)) {
    throw new Error('Cannot capture this page');
  }
  const dataUrl = await chrome.tabs.captureVisibleTab(null, { format: 'png' });
  await ensureContentScript(tabId);
  const response = await chrome.tabs.sendMessage(tabId, { type: 'start-pick-element', dataUrl });
  if (!response?.ok) throw new Error(response?.error || 'Content script busy');
}

function tabOrigin(tab) {
  try {
    if (tab?.url) return new URL(tab.url).origin;
  } catch {}
  return null;
}

function classifyCaptureFailure(err, tab) {
  const message = err?.message ?? String(err);
  const url = tab?.url;

  if (!url) {
    return { ok: false, error: message, code: 'permission_required', origin: null };
  }
  if (!/^https?:\/\//.test(url)) {
    return { ok: false, error: message, code: 'unsupported_page', origin: null };
  }
  if (/permission|activeTab|<all_urls>|Cannot access/i.test(message)) {
    return { ok: false, error: message, code: 'permission_required', origin: tabOrigin(tab) };
  }
  return { ok: false, error: message, code: 'other', origin: tabOrigin(tab) };
}

chrome.commands.onCommand.addListener(async (command) => {
  if (command !== 'toggle-capture') return;

  const [tab] = await chrome.tabs.query({ active: true, currentWindow: true });
  if (!tab?.id) return;

  startCaptureInTab(tab.id).catch((err) => {
    console.error('Capture failed:', err.message);
  });
});

function getServerUrl() {
  return new Promise((resolve) => {
    chrome.storage.sync.get({ serverUrl: DEFAULT_SERVER_URL }, (result) => {
      resolve(result.serverUrl);
    });
  });
}

function dataUrlToBlob(dataUrl) {
  const [header, base64] = dataUrl.split(',');
  const mime = header.match(/:(.*?);/)[1];
  const bytes = atob(base64);
  const arr = new Uint8Array(bytes.length);
  for (let i = 0; i < bytes.length; i++) {
    arr[i] = bytes.charCodeAt(i);
  }
  return new Blob([arr], { type: mime });
}

async function apiRequest(path, options = {}) {
  const serverUrl = await getServerUrl();
  const timeout = options.timeout || 5000;
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), timeout);

  try {
    const resp = await fetch(`${serverUrl}${path}`, {
      ...options,
      signal: controller.signal,
    });
    clearTimeout(timer);

    if (resp.status === 204) {
      return { ok: true };
    }

    let body;
    try {
      body = await resp.json();
    } catch {
      return { ok: false, error: `HTTP ${resp.status}` };
    }

    if (!resp.ok) {
      return { ok: false, error: body.error?.message || `HTTP ${resp.status}` };
    }

    return { ok: true, ...body };
  } catch (err) {
    clearTimeout(timer);
    if (err.name === 'AbortError') {
      return { ok: false, error: 'Request timed out' };
    }
    return { ok: false, error: 'Server unreachable' };
  }
}

chrome.runtime.onMessage.addListener((message, sender, sendResponse) => {
  switch (message.type) {
    case 'start-capture-from-panel': {
      (async () => {
        const [tab] = await chrome.tabs.query({ active: true, currentWindow: true });
        if (!tab?.id) {
          sendResponse({ ok: false, error: 'No active tab', code: 'no_active_tab', origin: null });
          return;
        }
        try {
          await startCaptureInTab(tab.id);
          sendResponse({ ok: true });
        } catch (err) {
          sendResponse(classifyCaptureFailure(err, tab));
        }
      })();
      return true;
    }

    case 'start-pick-element-from-panel': {
      (async () => {
        const [tab] = await chrome.tabs.query({ active: true, currentWindow: true });
        if (!tab?.id) {
          sendResponse({ ok: false, error: 'No active tab', code: 'no_active_tab', origin: null });
          return;
        }
        try {
          await startPickElementInTab(tab.id);
          sendResponse({ ok: true });
        } catch (err) {
          sendResponse(classifyCaptureFailure(err, tab));
        }
      })();
      return true;
    }

    case 'create-annotation': {
      const { annotation, imageDataUrl, hookContext } = message.data;

      (async () => {
        const serverUrl = await getServerUrl();
        const form = new FormData();
        form.append('annotation', JSON.stringify(annotation));

        if (imageDataUrl) {
          const blob = dataUrlToBlob(imageDataUrl);
          form.append('image', blob, 'screenshot.png');
        }

        if (hookContext) {
          if (hookContext.project) form.append('project', hookContext.project);
          if (hookContext.worktree) form.append('worktree', hookContext.worktree);
          if (hookContext.branch) form.append('branch', hookContext.branch);
          if (hookContext.commit) form.append('commit', hookContext.commit);
          if (hookContext.port) form.append('port', hookContext.port);
        }

        const controller = new AbortController();
        const timer = setTimeout(() => controller.abort(), 10000);

        try {
          const resp = await fetch(`${serverUrl}/api/annotations`, {
            method: 'POST',
            body: form,
            signal: controller.signal,
          });
          clearTimeout(timer);

          const body = await resp.json();

          if (!resp.ok) {
            sendResponse({ ok: false, error: body.error?.message || `HTTP ${resp.status}` });
            return;
          }

          chrome.runtime.sendMessage({ type: 'annotation-created', data: body.data }).catch(() => {});
          sendResponse({ ok: true, data: body.data });
        } catch (err) {
          clearTimeout(timer);
          if (err.name === 'AbortError') {
            sendResponse({ ok: false, error: 'Request timed out' });
          } else {
            sendResponse({ ok: false, error: 'Server unreachable' });
          }
        }
      })();
      return true;
    }

    case 'list-annotations': {
      const params = new URLSearchParams();
      const filters = message.data || {};
      for (const [key, val] of Object.entries(filters)) {
        if (val != null && val !== '') params.set(key, val);
      }
      const qs = params.toString();
      apiRequest(`/api/annotations${qs ? '?' + qs : ''}`).then(sendResponse);
      return true;
    }

    case 'get-annotation': {
      apiRequest(`/api/annotations/${message.data.id}`).then(sendResponse);
      return true;
    }

    case 'update-annotation': {
      const { id, annotation } = message.data;
      apiRequest(`/api/annotations/${id}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ annotation }),
      }).then(sendResponse);
      return true;
    }

    case 'delete-annotation': {
      apiRequest(`/api/annotations/${message.data.id}`, {
        method: 'DELETE',
      }).then(sendResponse);
      return true;
    }

    case 'check-health': {
      apiRequest('/health').then((result) => {
        sendResponse({ ok: result.ok });
      });
      return true;
    }

    case 'get-server-url':
      chrome.storage.sync.get({ serverUrl: DEFAULT_SERVER_URL }, (result) => {
        sendResponse({ url: result.serverUrl });
      });
      return true;

    case 'set-server-url':
      chrome.storage.sync.set({ serverUrl: message.url }, () => {
        sendResponse({ ok: true });
      });
      return true;
  }
});
