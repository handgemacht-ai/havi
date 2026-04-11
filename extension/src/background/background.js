const DEFAULT_SERVER_URL = 'http://localhost:8090';

chrome.sidePanel.setPanelBehavior({ openPanelOnActionClick: true });

chrome.commands.onCommand.addListener(async (command) => {
  if (command !== 'toggle-capture') return;

  const [tab] = await chrome.tabs.query({ active: true, currentWindow: true });
  if (!tab?.id) return;

  chrome.tabs.sendMessage(tab.id, { type: 'start-capture' });
});

chrome.runtime.onMessage.addListener((message, sender, sendResponse) => {
  switch (message.type) {
    case 'capture-visible-tab':
      chrome.tabs.captureVisibleTab(null, { format: 'png' })
        .then((dataUrl) => sendResponse({ dataUrl }))
        .catch((err) => sendResponse({ error: err.message }));
      return true;

    case 'create-annotation':
      console.log('[stub] create-annotation', message.data);
      sendResponse({ ok: true });
      return true;

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
