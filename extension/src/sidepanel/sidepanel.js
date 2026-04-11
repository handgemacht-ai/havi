const settingsToggle = document.getElementById('settings-toggle');
const settingsPanel = document.getElementById('settings-panel');
const serverUrlInput = document.getElementById('server-url-input');
const saveUrlBtn = document.getElementById('save-url-btn');
const settingsStatus = document.getElementById('settings-status');

function showStatus(message, type) {
  settingsStatus.textContent = message;
  settingsStatus.className = type;
  if (type === 'success') {
    setTimeout(() => {
      settingsStatus.textContent = '';
      settingsStatus.className = '';
    }, 2000);
  }
}

function isValidUrl(value) {
  return /^https?:\/\/.+/.test(value.trim());
}

settingsToggle.addEventListener('click', () => {
  const isOpen = settingsPanel.classList.toggle('open');
  settingsToggle.classList.toggle('active', isOpen);
});

saveUrlBtn.addEventListener('click', () => {
  const url = serverUrlInput.value.trim();

  if (!isValidUrl(url)) {
    serverUrlInput.classList.add('error');
    showStatus('URL must start with http:// or https://', 'error');
    return;
  }

  serverUrlInput.classList.remove('error');

  chrome.runtime.sendMessage({ type: 'set-server-url', url }, (response) => {
    if (chrome.runtime.lastError) {
      showStatus('Failed to save', 'error');
      return;
    }
    if (response?.ok) {
      showStatus('Saved', 'success');
    } else {
      showStatus('Failed to save', 'error');
    }
  });
});

serverUrlInput.addEventListener('input', () => {
  serverUrlInput.classList.remove('error');
  settingsStatus.textContent = '';
  settingsStatus.className = '';
});

chrome.runtime.sendMessage({ type: 'get-server-url' }, (response) => {
  if (chrome.runtime.lastError) return;
  if (response?.url) {
    serverUrlInput.value = response.url;
  }
});
