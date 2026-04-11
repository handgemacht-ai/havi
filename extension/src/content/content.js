(function () {
  'use strict';

  let state = 'idle';

  chrome.runtime.onMessage.addListener((message) => {
    if (message.type === 'start-capture' && state === 'idle') {
      startCapture();
    }
  });

  function setState(newState) {
    state = newState;
  }

  function startCapture() {
    setState('capturing');
    chrome.runtime.sendMessage({ type: 'capture-visible-tab' }, (response) => {
      if (response?.error) {
        console.error('captureVisibleTab failed:', response.error);
        setState('idle');
        return;
      }
      initRegionSelection(response.dataUrl);
    });
  }

  function initRegionSelection(dataUrl) {
    // Implemented in ann-2tl.2
    console.log('[placeholder] region selection', dataUrl?.slice(0, 50));
    setState('idle');
  }
})();
