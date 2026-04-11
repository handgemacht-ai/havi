const DEFAULT_SERVER_URL = 'http://localhost:8090';

chrome.sidePanel.setPanelBehavior({ openPanelOnActionClick: true });

async function ensureContentScript(tabId) {
  try {
    await chrome.tabs.sendMessage(tabId, { type: 'ping' });
  } catch {
    await chrome.scripting.executeScript({
      target: { tabId },
      files: ['assets/fabric.min.js', 'assets/cropper.min.js', 'src/content/content.js'],
    });
    await chrome.scripting.insertCSS({
      target: { tabId },
      files: ['assets/cropper.min.css', 'src/content/content.css'],
    });
  }
}

async function startCaptureInTab(tabId) {
  const tab = await chrome.tabs.get(tabId);
  if (!tab.url || !/^https?:\/\//.test(tab.url)) return;
  try {
    const dataUrl = await chrome.tabs.captureVisibleTab(null, { format: 'png' });
    await ensureContentScript(tabId);
    await chrome.tabs.sendMessage(tabId, { type: 'start-capture', dataUrl });
  } catch (err) {
    console.error('captureVisibleTab failed:', err.message);
  }
}

chrome.commands.onCommand.addListener(async (command) => {
  if (command !== 'toggle-capture') return;

  const [tab] = await chrome.tabs.query({ active: true, currentWindow: true });
  if (!tab?.id) return;

  startCaptureInTab(tab.id);
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
      chrome.tabs.query({ active: true, currentWindow: true }).then(([tab]) => {
        if (tab?.id) startCaptureInTab(tab.id);
      });
      return false;
    }

    case 'create-annotation': {
      const { annotation, imageDataUrl } = message.data;

      (async () => {
        const serverUrl = await getServerUrl();
        const form = new FormData();
        form.append('annotation', JSON.stringify(annotation));

        if (imageDataUrl) {
          const blob = dataUrlToBlob(imageDataUrl);
          form.append('image', blob, 'screenshot.png');
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
