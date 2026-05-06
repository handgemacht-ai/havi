const captureBtn = document.getElementById('capture-btn');
const captureBtnIcon = captureBtn.querySelector('svg').cloneNode(true);
const pickElementBtn = document.getElementById('pick-element-btn');
const pickElementBtnIcon = pickElementBtn.querySelector('svg').cloneNode(true);
const settingsToggle = document.getElementById('settings-toggle');
const settingsPanel = document.getElementById('settings-panel');
const serverUrlInput = document.getElementById('server-url-input');
const saveUrlBtn = document.getElementById('save-url-btn');
const settingsStatus = document.getElementById('settings-status');
const statusDot = document.getElementById('status-dot');
const statusLabel = document.getElementById('status-label');
const connectionBanner = document.getElementById('connection-banner');
const scopeTrigger = document.getElementById('scope-trigger');
const scopeLabel = document.getElementById('scope-label');
const scopeDropdown = document.getElementById('scope-dropdown');
const annotationList = document.getElementById('annotation-list');
const emptyState = document.getElementById('empty-state');
const emptyTitle = document.getElementById('empty-title');
const emptyBody = document.getElementById('empty-body');
const filterBar = document.getElementById('filter-bar');
const filterButtons = filterBar.querySelectorAll('.filter-btn');
const filterCounts = {
  all: filterBar.querySelector('[data-count="all"]'),
  open: filterBar.querySelector('[data-count="open"]'),
  resolved: filterBar.querySelector('[data-count="resolved"]'),
};
const listMeta = document.getElementById('list-meta');
const listMetaState = document.getElementById('list-meta-state');
const listMetaCount = document.getElementById('list-meta-count');
const captureAlert = document.getElementById('capture-alert');
const captureAlertMessage = document.getElementById('capture-alert-message');
const captureAlertAction = document.getElementById('capture-alert-action');
const captureAlertDismiss = document.getElementById('capture-alert-dismiss');

let serverUrl = 'http://localhost:8090';
let currentFilter = '';
let expandedId = null;
let editingId = null;
let annotations = [];
let captureMode = null;
let scopeOverride = null;
let activeScopeDomain = null;
let recentDomains = [];
let currentTabDomain = null;

const BROAD_ORIGIN_PATTERNS = ['<all_urls>'];

// --- Capture button state ---

function resetCaptureState() {
  captureMode = null;
  captureBtn.textContent = '';
  captureBtn.appendChild(captureBtnIcon.cloneNode(true));
  captureBtn.appendChild(document.createTextNode('Capture region'));
  captureBtn.classList.remove('cancel');
  pickElementBtn.textContent = '';
  pickElementBtn.appendChild(pickElementBtnIcon.cloneNode(true));
  pickElementBtn.classList.remove('cancel');
}

function enterCancelMode(which) {
  if (which === 'capture') {
    captureMode = 'capture';
    captureBtn.textContent = '';
    captureBtn.appendChild(captureBtnIcon.cloneNode(true));
    captureBtn.appendChild(document.createTextNode('Cancel capture'));
    captureBtn.classList.add('cancel');
  } else {
    captureMode = 'pick-element';
    pickElementBtn.textContent = '';
    pickElementBtn.appendChild(pickElementBtnIcon.cloneNode(true));
    const span = document.createElement('span');
    span.textContent = 'Cancel capture';
    pickElementBtn.appendChild(span);
    pickElementBtn.classList.add('cancel');
  }
}

function cancelActiveCapture() {
  chrome.tabs.query({ active: true, currentWindow: true }, ([tab]) => {
    if (tab?.id) {
      chrome.tabs.sendMessage(tab.id, { type: 'cancel-capture' }).catch(() => {
        resetCaptureState();
      });
    } else {
      resetCaptureState();
    }
  });
}

// --- Capture alerts ---

function hideCaptureAlert() {
  captureAlert.classList.add('hidden');
  captureAlertAction.classList.add('hidden');
  captureAlertAction.onclick = null;
}

function showCaptureAlert(message, action) {
  captureAlertMessage.textContent = message;
  if (action) {
    captureAlertAction.textContent = action.label;
    captureAlertAction.classList.remove('hidden');
    captureAlertAction.onclick = action.onClick;
  } else {
    captureAlertAction.classList.add('hidden');
    captureAlertAction.onclick = null;
  }
  captureAlert.classList.remove('hidden');
}

captureAlertDismiss.addEventListener('click', hideCaptureAlert);

function startCaptureRequest(messageType, label) {
  hideCaptureAlert();
  chrome.runtime.sendMessage({ type: messageType }, (response) => {
    if (chrome.runtime.lastError || !response?.ok) {
      resetCaptureState();
      handleCaptureFailure(messageType, label, response);
    }
  });
}

function handleCaptureFailure(messageType, label, response) {
  const error = response?.error || chrome.runtime.lastError?.message || 'no response';
  const code = response?.code;

  if (code === 'permission_required') {
    showCaptureAlert(
      `${label} needs to read pages you visit so it can take a screenshot. Grant access and HAVI will retry.`,
      {
        label: 'Grant access',
        onClick: () => requestPermissionAndRetry(messageType, label),
      },
    );
    return;
  }

  if (code === 'unsupported_page') {
    showCaptureAlert(`${label} is not available on this page (chrome:// and similar URLs are blocked).`);
    return;
  }

  showCaptureAlert(`${label} failed: ${error}`);
}

function requestPermissionAndRetry(messageType, label) {
  chrome.permissions.request({ origins: BROAD_ORIGIN_PATTERNS }, (granted) => {
    if (chrome.runtime.lastError) {
      showCaptureAlert(`Could not request permission: ${chrome.runtime.lastError.message}`);
      return;
    }
    if (!granted) {
      showCaptureAlert(`${label} needs site access. Permission was not granted.`);
      return;
    }
    enterCancelMode(messageType === 'start-capture-from-panel' ? 'capture' : 'pick-element');
    startCaptureRequest(messageType, label);
  });
}

captureBtn.addEventListener('click', () => {
  if (captureMode) {
    cancelActiveCapture();
    return;
  }
  enterCancelMode('capture');
  startCaptureRequest('start-capture-from-panel', 'Capture');
});

pickElementBtn.addEventListener('click', () => {
  if (captureMode) {
    cancelActiveCapture();
    return;
  }
  enterCancelMode('pick-element');
  startCaptureRequest('start-pick-element-from-panel', 'Pick element');
});

resetCaptureState();

// --- Settings ---

function showStatus(message, type) {
  settingsStatus.textContent = message;
  settingsStatus.className = `setting-hint${type ? ' ' + type : ''}`;
  if (type === 'success') {
    setTimeout(() => {
      settingsStatus.textContent = 'Non-localhost hosts will request browser permission.';
      settingsStatus.className = 'setting-hint';
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

saveUrlBtn.addEventListener('click', async () => {
  const url = serverUrlInput.value.trim();

  if (!isValidUrl(url)) {
    serverUrlInput.classList.add('invalid');
    showStatus('URL must start with http:// or https://', 'error');
    return;
  }

  serverUrlInput.classList.remove('invalid');

  try {
    const parsed = new URL(url);
    const isLocal = parsed.hostname === 'localhost' || parsed.hostname === '127.0.0.1';

    if (!isLocal) {
      const granted = await chrome.permissions.request({
        origins: [`${parsed.origin}/*`],
      });
      if (!granted) {
        showStatus('Permission required to connect to this server', 'error');
        return;
      }
    }
  } catch {
    showStatus('Invalid URL', 'error');
    return;
  }

  chrome.runtime.sendMessage({ type: 'set-server-url', url }, (response) => {
    if (chrome.runtime.lastError) {
      showStatus('Failed to save', 'error');
      return;
    }
    if (response?.ok) {
      serverUrl = url;
      showStatus('Saved', 'success');
      fetchAnnotations();
    } else {
      showStatus('Failed to save', 'error');
    }
  });
});

serverUrlInput.addEventListener('input', () => {
  serverUrlInput.classList.remove('invalid');
});

// --- Health Check ---

function setStatus(connected) {
  statusDot.classList.toggle('disconnected', !connected);
  statusLabel.textContent = connected ? 'HAVI · ready' : 'Server unreachable';
  connectionBanner.classList.toggle('hidden', connected);
}

function checkHealth() {
  chrome.runtime.sendMessage({ type: 'check-health' }, (response) => {
    if (chrome.runtime.lastError || !response?.ok) {
      setStatus(false);
    } else {
      setStatus(true);
    }
  });
}

// --- Helpers ---

function getComment(ann) {
  const body = ann.annotation?.body;
  if (!Array.isArray(body)) return '';
  const textBody = body.find((b) => b.type === 'TextualBody' && b.purpose === 'commenting');
  return textBody?.value || '';
}

function getElementText(ann) {
  const body = ann.annotation?.body;
  if (!Array.isArray(body)) return '';
  const desc = body.find((b) => b.type === 'TextualBody' && b.purpose === 'describing');
  return desc?.value || '';
}

function getCssSelector(ann) {
  const selectors = ann.annotation?.target?.selector;
  if (!Array.isArray(selectors)) return '';
  const css = selectors.find((s) => s.type === 'CssSelector');
  return css?.value || '';
}

function getViewport(ann) {
  const stateVal = ann.annotation?.target?.state?.value;
  if (!stateVal) return null;
  const match = stateVal.match(/viewport=(\S+)/);
  return match ? match[1] : null;
}

function getViewportClass(viewport) {
  if (!viewport) return null;
  const w = parseInt(viewport.split('x')[0], 10);
  if (Number.isNaN(w)) return null;
  if (w <= 480) return 'mobile';
  if (w <= 1024) return 'tablet';
  return 'desktop';
}

function timeAgo(dateStr) {
  const diff = Date.now() - new Date(dateStr).getTime();
  const mins = Math.floor(diff / 60000);
  if (mins < 1) return 'just now';
  if (mins < 60) return `${mins}m ago`;
  const hours = Math.floor(mins / 60);
  if (hours < 24) return `${hours}h ago`;
  const days = Math.floor(hours / 24);
  return `${days}d ago`;
}

const SCOPE_STORAGE_KEY = 'havi.scope';

function tabDomain() {
  return new Promise((resolve) => {
    chrome.tabs.query({ active: true, currentWindow: true }, (tabs) => {
      const url = tabs[0]?.url;
      if (!url) return resolve(null);
      try {
        const parsed = new URL(url);
        if (parsed.protocol !== 'http:' && parsed.protocol !== 'https:') return resolve(null);
        resolve(parsed.host || null);
      } catch {
        resolve(null);
      }
    });
  });
}

async function refreshCurrentTabDomain() {
  currentTabDomain = await tabDomain();
}

async function rememberDomain(domain) {
  if (!domain) return;
  try {
    await chrome.storage.session.set({ [SCOPE_STORAGE_KEY + '.lastTabDomain']: domain });
  } catch {}
}

async function recallLastTabDomain() {
  try {
    const obj = await chrome.storage.session.get(SCOPE_STORAGE_KEY + '.lastTabDomain');
    return obj[SCOPE_STORAGE_KEY + '.lastTabDomain'] || null;
  } catch {
    return null;
  }
}

async function loadScopeOverride() {
  try {
    const obj = await chrome.storage.session.get(SCOPE_STORAGE_KEY);
    return obj[SCOPE_STORAGE_KEY] || null;
  } catch {
    return null;
  }
}

async function saveScopeOverride(scope) {
  try {
    if (scope) await chrome.storage.session.set({ [SCOPE_STORAGE_KEY]: scope });
    else await chrome.storage.session.remove(SCOPE_STORAGE_KEY);
  } catch {}
}

function initials(name) {
  if (!name) return '?';
  const parts = name.trim().split(/\s+/);
  return (parts[0]?.[0] || '?').toUpperCase();
}

// --- DOM Construction (safe, no innerHTML with user data) ---

function svgEl(tag, attrs) {
  const node = document.createElementNS('http://www.w3.org/2000/svg', tag);
  if (attrs) for (const [k, v] of Object.entries(attrs)) node.setAttribute(k, v);
  return node;
}

function svg(attrs, ...children) {
  const node = svgEl('svg', attrs);
  for (const child of children) node.appendChild(child);
  return node;
}

function el(tag, attrs, ...children) {
  const node = document.createElement(tag);
  if (attrs) {
    for (const [key, val] of Object.entries(attrs)) {
      if (key === 'className') node.className = val;
      else if (key === 'dataset') Object.assign(node.dataset, val);
      else if (key.startsWith('on')) node.addEventListener(key.slice(2).toLowerCase(), val);
      else node.setAttribute(key, val);
    }
  }
  for (const child of children) {
    if (child == null) continue;
    if (typeof child === 'string') node.appendChild(document.createTextNode(child));
    else node.appendChild(child);
  }
  return node;
}

function viewportIcon(klass) {
  if (klass === 'mobile') {
    return svg(
      { width: '10', height: '10', viewBox: '0 0 24 24', fill: 'none', stroke: 'currentColor', 'stroke-width': '2', 'stroke-linecap': 'round', 'stroke-linejoin': 'round' },
      svgEl('rect', { x: '5', y: '2', width: '14', height: '20', rx: '2', ry: '2' }),
      svgEl('line', { x1: '12', y1: '18', x2: '12.01', y2: '18' }),
    );
  }
  if (klass === 'tablet') {
    return svg(
      { width: '10', height: '10', viewBox: '0 0 24 24', fill: 'none', stroke: 'currentColor', 'stroke-width': '2', 'stroke-linecap': 'round', 'stroke-linejoin': 'round' },
      svgEl('rect', { x: '4', y: '2', width: '16', height: '20', rx: '2', ry: '2' }),
      svgEl('line', { x1: '12', y1: '18', x2: '12.01', y2: '18' }),
    );
  }
  return svg(
    { width: '10', height: '10', viewBox: '0 0 24 24', fill: 'none', stroke: 'currentColor', 'stroke-width': '2', 'stroke-linecap': 'round', 'stroke-linejoin': 'round' },
    svgEl('rect', { x: '2', y: '3', width: '20', height: '14', rx: '2', ry: '2' }),
    svgEl('line', { x1: '8', y1: '21', x2: '16', y2: '21' }),
    svgEl('line', { x1: '12', y1: '17', x2: '12', y2: '21' }),
  );
}

function branchIcon() {
  return svg(
    { width: '10', height: '10', viewBox: '0 0 24 24', fill: 'none', stroke: 'currentColor', 'stroke-width': '2', 'stroke-linecap': 'round', 'stroke-linejoin': 'round' },
    svgEl('line', { x1: '6', y1: '3', x2: '6', y2: '15' }),
    svgEl('circle', { cx: '18', cy: '6', r: '3' }),
    svgEl('circle', { cx: '6', cy: '18', r: '3' }),
    svgEl('path', { d: 'M18 9a9 9 0 0 1-9 9' }),
  );
}

function checkIcon(size) {
  return svg(
    { width: String(size), height: String(size), viewBox: '0 0 24 24', fill: 'none', stroke: 'currentColor', 'stroke-width': '3', 'stroke-linecap': 'round', 'stroke-linejoin': 'round' },
    svgEl('polyline', { points: '20 6 9 17 4 12' }),
  );
}

function chevronIcon() {
  return svg(
    { class: 'card-chevron', width: '14', height: '14', viewBox: '0 0 24 24', fill: 'none', stroke: 'currentColor', 'stroke-width': '2', 'stroke-linecap': 'round', 'stroke-linejoin': 'round' },
    svgEl('polyline', { points: '6 9 12 15 18 9' }),
  );
}

// --- Fetch & Render ---

async function resolveActiveDomain() {
  if (scopeOverride && scopeOverride.kind === 'all') return null;
  if (scopeOverride && scopeOverride.kind === 'domain') return scopeOverride.value;
  const live = await tabDomain();
  if (live) {
    rememberDomain(live);
    return live;
  }
  return await recallLastTabDomain();
}

function renderScopeLabel(domain) {
  scopeLabel.classList.remove('muted', 'all-scope');
  if (scopeOverride && scopeOverride.kind === 'all') {
    scopeLabel.textContent = 'All domains';
    scopeLabel.classList.add('all-scope');
    return;
  }
  if (domain) {
    scopeLabel.textContent = domain;
    return;
  }
  scopeLabel.textContent = 'No domain detected';
  scopeLabel.classList.add('muted');
}

async function fetchAnnotations() {
  const domain = await resolveActiveDomain();
  activeScopeDomain = domain;
  renderScopeLabel(domain);

  const filters = {};
  if (domain) filters.domain = domain;

  chrome.runtime.sendMessage({ type: 'list-annotations', data: filters }, (response) => {
    if (chrome.runtime.lastError || !response?.ok) {
      annotations = [];
      renderList('error');
      return;
    }
    annotations = response.data || [];
    renderList();
  });
}

function refreshRecentDomains() {
  chrome.runtime.sendMessage({ type: 'get-scopes' }, (response) => {
    if (chrome.runtime.lastError || !response?.ok) return;
    recentDomains = response.data?.domains || [];
    if (!scopeDropdown.classList.contains('hidden')) renderScopeDropdown();
  });
}

function renderScopeDropdown() {
  scopeDropdown.textContent = '';

  const overrideKind = scopeOverride?.kind || 'tab';
  const overrideValue = scopeOverride?.value || null;

  const candidates = new Set();
  if (currentTabDomain) candidates.add(currentTabDomain);
  if (activeScopeDomain) candidates.add(activeScopeDomain);
  for (const d of recentDomains) candidates.add(d);

  const recentSection = document.createElement('div');
  recentSection.className = 'dropdown-section';
  const recentLabel = document.createElement('div');
  recentLabel.className = 'dropdown-label';
  recentLabel.textContent = 'RECENT DOMAINS';
  recentSection.appendChild(recentLabel);

  if (candidates.size === 0) {
    const empty = document.createElement('div');
    empty.className = 'dropdown-empty';
    empty.textContent = 'No annotated domains yet.';
    recentSection.appendChild(empty);
  } else {
    for (const domain of candidates) {
      const item = document.createElement('button');
      item.type = 'button';
      const isActive = overrideKind === 'domain'
        ? overrideValue === domain
        : overrideKind === 'tab' && activeScopeDomain === domain;
      item.className = 'dropdown-item' + (isActive ? ' active' : '');
      item.appendChild(svg(
        { width: '12', height: '12', viewBox: '0 0 24 24', fill: 'none', stroke: 'currentColor', 'stroke-width': '2', 'stroke-linecap': 'round', 'stroke-linejoin': 'round' },
        svgEl('circle', { cx: '12', cy: '12', r: '10' }),
        svgEl('line', { x1: '2', y1: '12', x2: '22', y2: '12' }),
        svgEl('path', { d: 'M12 2a15.3 15.3 0 0 1 4 10 15.3 15.3 0 0 1-4 10 15.3 15.3 0 0 1-4-10 15.3 15.3 0 0 1 4-10z' }),
      ));
      item.appendChild(document.createTextNode(domain));
      item.addEventListener('click', () => selectScope({ kind: 'domain', value: domain }));
      recentSection.appendChild(item);
    }
  }

  scopeDropdown.appendChild(recentSection);

  const allSection = document.createElement('div');
  allSection.className = 'dropdown-section';
  const allItem = document.createElement('button');
  allItem.type = 'button';
  allItem.className = 'dropdown-item special' + (overrideKind === 'all' ? ' active' : '');
  allItem.appendChild(svg(
    { width: '12', height: '12', viewBox: '0 0 24 24', fill: 'none', stroke: 'currentColor', 'stroke-width': '2', 'stroke-linecap': 'round', 'stroke-linejoin': 'round' },
    svgEl('polygon', { points: '12 2 2 7 12 12 22 7 12 2' }),
    svgEl('polyline', { points: '2 17 12 22 22 17' }),
    svgEl('polyline', { points: '2 12 12 17 22 12' }),
  ));
  allItem.appendChild(document.createTextNode('All domains'));
  allItem.addEventListener('click', () => selectScope({ kind: 'all' }));
  allSection.appendChild(allItem);
  scopeDropdown.appendChild(allSection);
}

async function selectScope(scope) {
  if (scope.kind === 'domain' && !scopeOverride && scope.value === activeScopeDomain) {
    closeScopeDropdown();
    return;
  }
  scopeOverride = scope;
  await saveScopeOverride(scope);
  closeScopeDropdown();
  fetchAnnotations();
}

function openScopeDropdown() {
  renderScopeDropdown();
  scopeDropdown.classList.remove('hidden');
  scopeTrigger.setAttribute('aria-expanded', 'true');
  refreshCurrentTabDomain().then(() => {
    if (!scopeDropdown.classList.contains('hidden')) renderScopeDropdown();
  });
  refreshRecentDomains();
}

function closeScopeDropdown() {
  scopeDropdown.classList.add('hidden');
  scopeTrigger.setAttribute('aria-expanded', 'false');
}

scopeTrigger.addEventListener('click', (e) => {
  e.stopPropagation();
  if (scopeDropdown.classList.contains('hidden')) openScopeDropdown();
  else closeScopeDropdown();
});

document.addEventListener('click', (e) => {
  if (scopeDropdown.classList.contains('hidden')) return;
  if (scopeDropdown.contains(e.target) || scopeTrigger.contains(e.target)) return;
  closeScopeDropdown();
});

document.addEventListener('keydown', (e) => {
  if (e.key === 'Escape' && !scopeDropdown.classList.contains('hidden')) closeScopeDropdown();
});

function updateCounts() {
  filterCounts.all.textContent = String(annotations.length);
  filterCounts.open.textContent = String(annotations.filter((a) => a.state === 'open').length);
  filterCounts.resolved.textContent = String(annotations.filter((a) => a.state === 'resolved').length);
}

function visibleAnnotations() {
  if (!currentFilter) return annotations;
  return annotations.filter((a) => a.state === currentFilter);
}

function renderList(errorState) {
  const existing = annotationList.querySelectorAll('.annotation-card');
  existing.forEach((c) => c.remove());

  updateCounts();

  if (errorState === 'error') {
    listMeta.classList.add('hidden');
    emptyState.classList.remove('hidden');
    emptyTitle.textContent = 'Cannot reach server';
    emptyBody.textContent = 'Check the URL in settings and confirm the HAVI server is running.';
    return;
  }

  const visible = visibleAnnotations();

  if (visible.length === 0) {
    listMeta.classList.add('hidden');
    emptyState.classList.remove('hidden');
    if (currentFilter) {
      emptyTitle.textContent = 'No matches';
      emptyBody.textContent = `No ${currentFilter} annotations on this domain.`;
    } else {
      emptyTitle.textContent = 'No annotations yet';
      emptyBody.textContent = '';
      emptyBody.appendChild(document.createTextNode('Press '));
      emptyBody.appendChild(el('kbd', null, 'Ctrl'));
      emptyBody.appendChild(el('kbd', null, 'Shift'));
      emptyBody.appendChild(el('kbd', null, 'A'));
      emptyBody.appendChild(document.createTextNode(' or use '));
      emptyBody.appendChild(el('strong', null, 'Capture region'));
      emptyBody.appendChild(document.createTextNode(' above to grab your first one.'));
    }
    return;
  }

  emptyState.classList.add('hidden');
  listMeta.classList.remove('hidden');
  listMetaState.textContent = currentFilter ? currentFilter : 'All';
  listMetaCount.textContent = String(visible.length);

  for (const ann of visible) {
    annotationList.appendChild(createCard(ann));
  }
}

function createCard(ann) {
  const comment = getComment(ann);
  const viewport = getViewport(ann);
  const viewportClass = getViewportClass(viewport);
  const elementText = getElementText(ann);
  const cssSelector = getCssSelector(ann);
  const isExpanded = expandedId === ann.id;
  const creatorName = ann.creator || ann.annotation?.creator?.name || '';
  const branch = ann.branch || '';
  const worktree = ann.worktree || '';

  // Thumbnail
  const thumbImg = el('img', {
    className: 'card-thumb',
    src: `${serverUrl}/api/annotations/${ann.id}/image`,
    alt: '',
    loading: 'lazy',
  });
  const thumbPlaceholder = el('div', { className: 'card-thumb-placeholder', style: 'display:none' });
  thumbPlaceholder.appendChild(svg(
    { width: '18', height: '18', viewBox: '0 0 24 24', fill: 'none', stroke: 'currentColor', 'stroke-width': '1.5' },
    svgEl('rect', { x: '3', y: '3', width: '18', height: '18', rx: '2' }),
    svgEl('circle', { cx: '8.5', cy: '8.5', r: '1.5' }),
    svgEl('path', { d: 'm21 15-5-5L5 21' }),
  ));
  thumbImg.addEventListener('error', () => {
    thumbImg.style.display = 'none';
    thumbPlaceholder.style.display = 'flex';
  });

  const thumbWrap = el('div', { className: 'card-thumb-wrap' }, thumbImg, thumbPlaceholder);
  if (ann.state === 'resolved') {
    thumbWrap.appendChild(el('div', { className: 'thumb-resolved-mask' }, checkIcon(14)));
  }

  // Top row: state pill, viewport chip, time
  const topRow = el('div', { className: 'card-top-row' },
    el('span', { className: `state-pill state-${ann.state}` }, ann.state),
    viewport ? el('span', { className: 'vp-chip' }, viewportIcon(viewportClass), viewport) : null,
    el('span', { className: 'card-time' }, timeAgo(ann.created_at)),
  );

  // Comment
  const commentEl = el('p', { className: 'card-comment' }, comment);

  // Meta row: creator, branch
  const metaRow = el('div', { className: 'card-meta-row' });
  if (creatorName) {
    metaRow.appendChild(el('span', { className: 'meta-chip' },
      el('span', { className: 'mini-avatar' }, initials(creatorName)),
      creatorName,
    ));
  }
  if (branch) {
    metaRow.appendChild(el('span', { className: 'meta-chip' }, branchIcon(), branch));
  }

  const cardBody = el('div', { className: 'card-body' }, topRow, commentEl, metaRow);
  const chevron = chevronIcon();
  if (isExpanded) chevron.classList.add('expanded');

  const summary = el('div', { className: 'card-summary', onClick: () => toggleExpand(ann.id) },
    thumbWrap,
    cardBody,
    chevron,
  );

  // Detail view
  const detailImg = el('img', {
    className: 'detail-image',
    src: `${serverUrl}/api/annotations/${ann.id}/image`,
    alt: 'Screenshot',
    loading: 'lazy',
  });
  detailImg.addEventListener('error', () => { detailImg.style.display = 'none'; });

  const commentWrap = el('div', { className: 'detail-comment-wrap', id: `comment-wrap-${ann.id}` },
    el('div', { className: 'detail-comment-head' },
      el('span', { className: 'eyebrow' }, 'COMMENT'),
      el('button', { className: 'link-btn', onClick: (e) => { e.stopPropagation(); startEdit(ann); } }, 'Edit'),
    ),
    el('p', { className: 'detail-comment' }, comment || ''),
  );

  // Element block
  const elementBlock = el('div', { className: 'detail-element' });
  if (cssSelector || elementText) {
    elementBlock.appendChild(el('span', { className: 'eyebrow' }, 'ELEMENT'));
    if (cssSelector) {
      elementBlock.appendChild(el('code', { className: 'detail-selector' }, cssSelector));
    }
    if (elementText) {
      const truncated = elementText.length > 280 ? elementText.slice(0, 280) + '…' : elementText;
      elementBlock.appendChild(el('pre', { className: 'detail-element-text' }, truncated));
    }
  }

  // Metadata grid
  const metaDl = el('dl', { className: 'meta-dl' });
  const entries = [
    ['DOMAIN', ann.domain],
    ['CREATOR', creatorName],
    ['CREATED', new Date(ann.created_at).toLocaleString()],
    ['STATE', ann.state],
  ];
  if (viewport) entries.push(['VIEWPORT', viewport]);
  if (ann.motivation) entries.push(['MOTIVATION', ann.motivation]);
  if (ann.project) entries.push(['PROJECT', ann.project]);
  if (worktree) entries.push(['WORKTREE', worktree]);
  if (branch) entries.push(['BRANCH', branch]);
  for (const [k, v] of entries) {
    if (v == null || v === '') continue;
    metaDl.appendChild(el('div', null,
      el('dt', null, k),
      el('dd', null, String(v)),
    ));
  }

  const deleteConfirm = el('div', { className: 'confirm-strip hidden', id: `delete-confirm-${ann.id}` },
    el('span', null, "Delete this annotation? This can't be undone."),
    el('div', { className: 'confirm-actions' },
      el('button', { className: 'btn btn-ghost btn-sm', onClick: (e) => { e.stopPropagation(); hideDeleteConfirm(ann.id); } }, 'Cancel'),
      el('button', { className: 'btn btn-danger btn-sm', onClick: (e) => { e.stopPropagation(); deleteAnnotation(ann.id); } }, 'Delete'),
    ),
  );

  const actions = el('div', { className: 'detail-actions' },
    el('button', { className: 'btn btn-outline btn-sm', onClick: (e) => { e.stopPropagation(); startEdit(ann); } }, 'Edit'),
    el('div', { className: 'action-spacer' }),
    el('button', { className: 'icon-btn sm danger', onClick: (e) => { e.stopPropagation(); showDeleteConfirm(ann.id); }, title: 'Delete' },
      svg(
        { width: '12', height: '12', viewBox: '0 0 24 24', fill: 'none', stroke: 'currentColor', 'stroke-width': '2', 'stroke-linecap': 'round', 'stroke-linejoin': 'round' },
        svgEl('polyline', { points: '3 6 5 6 21 6' }),
        svgEl('path', { d: 'M19 6l-2 14a2 2 0 0 1-2 2H9a2 2 0 0 1-2-2L5 6' }),
        svgEl('path', { d: 'M10 11v6' }),
        svgEl('path', { d: 'M14 11v6' }),
      ),
    ),
  );

  const detail = el('div', { className: `card-detail${isExpanded ? ' open' : ''}` },
    el('div', { className: 'detail-content' },
      detailImg,
      commentWrap,
      (cssSelector || elementText) ? elementBlock : null,
      metaDl,
      actions,
      deleteConfirm,
    ),
  );

  return el('div', { className: `annotation-card${isExpanded ? ' expanded' : ''}`, dataset: { id: ann.id } }, summary, detail);
}

// --- Expand/Collapse ---

function toggleExpand(id) {
  if (editingId) return;

  const wasExpanded = expandedId === id;
  expandedId = wasExpanded ? null : id;

  annotationList.querySelectorAll('.annotation-card').forEach((card) => {
    const detail = card.querySelector('.card-detail');
    const chevron = card.querySelector('.card-chevron');
    const isTarget = card.dataset.id === id;

    if (isTarget && !wasExpanded) {
      detail.classList.add('open');
      chevron.classList.add('expanded');
      card.classList.add('expanded');
    } else {
      detail.classList.remove('open');
      chevron.classList.remove('expanded');
      card.classList.remove('expanded');
    }
  });
}

// --- Edit ---

function startEdit(ann) {
  editingId = ann.id;
  const wrap = document.getElementById(`comment-wrap-${ann.id}`);
  if (!wrap) return;
  wrap.textContent = '';

  const textarea = el('textarea', { className: 'edit-textarea' }, getComment(ann));
  const actions = el('div', { className: 'edit-actions' },
    el('button', { className: 'btn btn-ghost btn-sm', onClick: () => cancelEdit(ann) }, 'Cancel'),
    el('button', { className: 'btn btn-primary btn-sm', onClick: () => saveEdit(ann) }, 'Save'),
  );

  wrap.appendChild(el('span', { className: 'eyebrow' }, 'EDIT COMMENT'));
  wrap.appendChild(textarea);
  wrap.appendChild(actions);
  textarea.focus();
}

function renderViewComment(ann) {
  const wrap = document.getElementById(`comment-wrap-${ann.id}`);
  if (!wrap) return;
  wrap.textContent = '';
  wrap.appendChild(el('div', { className: 'detail-comment-head' },
    el('span', { className: 'eyebrow' }, 'COMMENT'),
    el('button', { className: 'link-btn', onClick: (e) => { e.stopPropagation(); startEdit(ann); } }, 'Edit'),
  ));
  wrap.appendChild(el('p', { className: 'detail-comment' }, getComment(ann) || ''));
}

function cancelEdit(ann) {
  editingId = null;
  renderViewComment(ann);
}

function saveEdit(ann) {
  const wrap = document.getElementById(`comment-wrap-${ann.id}`);
  const textarea = wrap.querySelector('.edit-textarea');
  const newValue = textarea.value.trim();

  if (!newValue) return;

  const origBody = ann.annotation.body || [];
  const hasComment = origBody.some((b) => b.type === 'TextualBody' && b.purpose === 'commenting');
  let updatedBody;
  if (hasComment) {
    updatedBody = origBody.map((b) => {
      if (b.type === 'TextualBody' && b.purpose === 'commenting') {
        return { ...b, value: newValue };
      }
      return b;
    });
  } else {
    updatedBody = [{ type: 'TextualBody', value: newValue, purpose: 'commenting' }, ...origBody];
  }

  chrome.runtime.sendMessage({
    type: 'update-annotation',
    data: { id: ann.id, annotation: { body: updatedBody } },
  }, (response) => {
    editingId = null;
    if (chrome.runtime.lastError || !response?.ok) {
      const w = document.getElementById(`comment-wrap-${ann.id}`);
      if (w) {
        w.appendChild(el('p', { className: 'edit-error' }, 'Failed to save. Try again.'));
      }
      return;
    }

    const idx = annotations.findIndex((a) => a.id === ann.id);
    if (idx >= 0) {
      annotations[idx] = response.data;
    }
    expandedId = ann.id;
    renderList();
  });
}

// --- Delete ---

function showDeleteConfirm(id) {
  document.getElementById(`delete-confirm-${id}`).classList.remove('hidden');
}

function hideDeleteConfirm(id) {
  document.getElementById(`delete-confirm-${id}`).classList.add('hidden');
}

function deleteAnnotation(id) {
  chrome.runtime.sendMessage({ type: 'delete-annotation', data: { id } }, (response) => {
    if (chrome.runtime.lastError || !response?.ok) return;
    annotations = annotations.filter((a) => a.id !== id);
    if (expandedId === id) expandedId = null;
    renderList();
  });
}

// --- Filter ---

filterBar.addEventListener('click', (e) => {
  const btn = e.target.closest('.filter-btn');
  if (!btn) return;

  filterButtons.forEach((b) => b.classList.remove('active'));
  btn.classList.add('active');
  currentFilter = btn.dataset.state || '';
  renderList();
});

// --- Auto-refresh ---

chrome.runtime.onMessage.addListener((message) => {
  if (message.type === 'annotation-created' && message.data) {
    annotations.unshift(message.data);
    renderList();
    refreshRecentDomains();
  }
  if (message.type === 'capture-ended') {
    resetCaptureState();
  }
});

// --- Init ---

(async () => {
  scopeOverride = await loadScopeOverride();
  await refreshCurrentTabDomain();
  chrome.runtime.sendMessage({ type: 'get-server-url' }, (response) => {
    if (chrome.runtime.lastError) return;
    if (response?.url) {
      serverUrl = response.url;
      serverUrlInput.value = response.url;
    }
    checkHealth();
    fetchAnnotations();
    refreshRecentDomains();
  });
})();

function onTabChanged() {
  const prev = currentTabDomain;
  refreshCurrentTabDomain().then(() => {
    if (currentTabDomain === prev) return;
    fetchAnnotations();
    if (!scopeDropdown.classList.contains('hidden')) renderScopeDropdown();
  });
}

chrome.tabs.onActivated.addListener(onTabChanged);

chrome.tabs.onUpdated.addListener((_tabId, changeInfo, tab) => {
  if (!tab?.active) return;
  if (changeInfo.url || changeInfo.status === 'complete') {
    onTabChanged();
  }
});

chrome.windows.onFocusChanged.addListener((windowId) => {
  if (windowId === chrome.windows.WINDOW_ID_NONE) return;
  onTabChanged();
});

setInterval(checkHealth, 30000);
