const captureBtn = document.getElementById('capture-btn');
const settingsToggle = document.getElementById('settings-toggle');
const settingsPanel = document.getElementById('settings-panel');
const serverUrlInput = document.getElementById('server-url-input');
const saveUrlBtn = document.getElementById('save-url-btn');
const settingsStatus = document.getElementById('settings-status');
const statusDot = document.getElementById('status-dot');
const connectionBanner = document.getElementById('connection-banner');
const annotationList = document.getElementById('annotation-list');
const emptyState = document.getElementById('empty-state');
const filterBar = document.getElementById('filter-bar');

let serverUrl = 'http://localhost:8090';
let currentFilter = '';
let expandedId = null;
let editingId = null;
let annotations = [];

// --- Settings ---

captureBtn.addEventListener('click', () => {
  chrome.runtime.sendMessage({ type: 'start-capture-from-panel' });
});

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
      serverUrl = url;
      showStatus('Saved', 'success');
      fetchAnnotations();
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

// --- Health Check ---

function checkHealth() {
  chrome.runtime.sendMessage({ type: 'check-health' }, (response) => {
    if (chrome.runtime.lastError || !response?.ok) {
      statusDot.classList.add('disconnected');
      connectionBanner.classList.remove('hidden');
    } else {
      statusDot.classList.remove('disconnected');
      connectionBanner.classList.add('hidden');
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

function getViewport(ann) {
  const stateVal = ann.annotation?.target?.state?.value;
  if (!stateVal) return null;
  const match = stateVal.match(/viewport=(\S+)/);
  return match ? match[1] : null;
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

function getCurrentDomain() {
  return new Promise((resolve) => {
    chrome.tabs.query({ active: true, currentWindow: true }, (tabs) => {
      if (!tabs[0]?.url) {
        resolve(null);
        return;
      }
      try {
        resolve(new URL(tabs[0].url).host);
      } catch {
        resolve(null);
      }
    });
  });
}

function escapeHtml(str) {
  const div = document.createElement('div');
  div.textContent = str;
  return div.innerHTML;
}

// --- DOM Construction (safe, no innerHTML with user data) ---

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
    if (typeof child === 'string') node.appendChild(document.createTextNode(child));
    else if (child) node.appendChild(child);
  }
  return node;
}

// --- Fetch & Render ---

async function fetchAnnotations() {
  const domain = await getCurrentDomain();
  const filters = {};
  if (domain) filters.domain = domain;
  if (currentFilter) filters.state = currentFilter;

  chrome.runtime.sendMessage({ type: 'list-annotations', data: filters }, (response) => {
    if (chrome.runtime.lastError) {
      showEmptyState('error');
      return;
    }
    if (!response?.ok) {
      showEmptyState('error');
      return;
    }
    annotations = response.data || [];
    renderList();
  });
}

function showEmptyState(type) {
  const cards = annotationList.querySelectorAll('.annotation-card');
  cards.forEach((c) => c.remove());

  emptyState.style.display = 'flex';
  const msg = emptyState.querySelector('p');
  const hint = document.getElementById('empty-hint');

  if (type === 'error') {
    msg.textContent = 'Cannot connect to server.';
    hint.textContent = 'Check settings.';
  } else if (type === 'filtered') {
    const label = currentFilter || 'matching';
    msg.textContent = `No ${label} annotations.`;
    hint.textContent = '';
  } else {
    msg.textContent = 'No annotations yet.';
    hint.textContent = '';
    hint.appendChild(document.createTextNode('Press '));
    const kbd = document.createElement('kbd');
    kbd.textContent = 'Ctrl+Shift+A';
    hint.appendChild(kbd);
    hint.appendChild(document.createTextNode(' to capture your first annotation.'));
  }
}

function renderList() {
  const existing = annotationList.querySelectorAll('.annotation-card');
  existing.forEach((c) => c.remove());

  if (annotations.length === 0) {
    showEmptyState(currentFilter ? 'filtered' : 'default');
    return;
  }

  emptyState.style.display = 'none';

  for (const ann of annotations) {
    annotationList.appendChild(createCard(ann));
  }
}

function createCard(ann) {
  const comment = getComment(ann);
  const viewport = getViewport(ann);
  const isExpanded = expandedId === ann.id;

  const thumbImg = el('img', {
    className: 'card-thumb',
    src: `${serverUrl}/api/annotations/${ann.id}/image`,
    alt: '',
    loading: 'lazy',
  });
  const thumbPlaceholder = el('div', { className: 'card-thumb-placeholder', style: 'display:none' });
  const phSvg = document.createElementNS('http://www.w3.org/2000/svg', 'svg');
  phSvg.setAttribute('width', '20');
  phSvg.setAttribute('height', '20');
  phSvg.setAttribute('viewBox', '0 0 24 24');
  phSvg.setAttribute('fill', 'none');
  phSvg.setAttribute('stroke', 'currentColor');
  phSvg.setAttribute('stroke-width', '1.5');
  ['rect:x=3:y=3:width=18:height=18:rx=2', 'circle:cx=8.5:cy=8.5:r=1.5'].forEach((spec) => {
    const [tag, ...attrs] = spec.split(':');
    const node = document.createElementNS('http://www.w3.org/2000/svg', tag);
    attrs.forEach((a) => { const [k, v] = a.split('='); node.setAttribute(k, v); });
    phSvg.appendChild(node);
  });
  const phPath = document.createElementNS('http://www.w3.org/2000/svg', 'path');
  phPath.setAttribute('d', 'm21 15-5-5L5 21');
  phSvg.appendChild(phPath);
  thumbPlaceholder.appendChild(phSvg);
  thumbImg.addEventListener('error', () => {
    thumbImg.style.display = 'none';
    thumbPlaceholder.style.display = 'flex';
  });

  const metaChips = el('div', { className: 'card-meta' },
    el('span', { className: `chip chip-state chip-${ann.state}` }, ann.state),
    ...(viewport ? [el('span', { className: 'chip' }, viewport)] : []),
    el('span', { className: 'chip chip-time' }, timeAgo(ann.created_at)),
  );

  const chevronSvg = document.createElementNS('http://www.w3.org/2000/svg', 'svg');
  chevronSvg.setAttribute('class', `card-chevron${isExpanded ? ' expanded' : ''}`);
  chevronSvg.setAttribute('width', '16');
  chevronSvg.setAttribute('height', '16');
  chevronSvg.setAttribute('viewBox', '0 0 16 16');
  chevronSvg.setAttribute('fill', 'none');
  chevronSvg.setAttribute('stroke', 'currentColor');
  chevronSvg.setAttribute('stroke-width', '2');
  chevronSvg.setAttribute('stroke-linecap', 'round');
  chevronSvg.setAttribute('stroke-linejoin', 'round');
  const chevronPath = document.createElementNS('http://www.w3.org/2000/svg', 'path');
  chevronPath.setAttribute('d', 'm4 6 4 4 4-4');
  chevronSvg.appendChild(chevronPath);

  const summary = el('div', { className: 'card-summary', onClick: () => toggleExpand(ann.id) },
    el('div', { className: 'card-thumb-wrap' }, thumbImg, thumbPlaceholder),
    el('div', { className: 'card-body' },
      el('p', { className: 'card-comment' }, comment.length > 80 ? comment.slice(0, 80) + '...' : comment),
      metaChips,
    ),
    chevronSvg,
  );

  const detailImg = el('img', {
    className: 'detail-image',
    src: `${serverUrl}/api/annotations/${ann.id}/image`,
    alt: 'Screenshot',
    loading: 'lazy',
  });
  detailImg.addEventListener('error', () => { detailImg.style.display = 'none'; });

  const commentWrap = el('div', { className: 'detail-comment-wrap', id: `comment-wrap-${ann.id}` },
    el('p', { className: 'detail-comment' }, comment),
  );

  const metaDl = document.createElement('dl');
  metaDl.className = 'detail-meta';
  const metaEntries = [
    ['Domain', ann.domain],
    ['Creator', ann.creator],
    ['Created', new Date(ann.created_at).toLocaleString()],
  ];
  if (viewport) metaEntries.push(['Viewport', viewport]);
  for (const [label, value] of metaEntries) {
    metaDl.appendChild(el('dt', null, label));
    metaDl.appendChild(el('dd', null, value));
  }
  const stateDt = el('dt', null, 'State');
  const stateDd = el('dd', null, el('span', { className: `chip chip-state chip-${ann.state}` }, ann.state));
  metaDl.appendChild(stateDt);
  metaDl.appendChild(stateDd);

  const deleteConfirm = el('div', { className: 'delete-confirm hidden', id: `delete-confirm-${ann.id}` },
    el('span', null, 'Delete this annotation?'),
    el('button', { className: 'btn-confirm-delete', onClick: (e) => { e.stopPropagation(); deleteAnnotation(ann.id); } }, 'Confirm'),
    el('button', { className: 'btn-cancel-delete', onClick: (e) => { e.stopPropagation(); hideDeleteConfirm(ann.id); } }, 'Cancel'),
  );

  const detail = el('div', { className: `card-detail${isExpanded ? ' open' : ''}` },
    detailImg,
    el('div', { className: 'detail-content' },
      commentWrap,
      metaDl,
      el('div', { className: 'detail-actions' },
        el('button', { className: 'btn-edit', onClick: (e) => { e.stopPropagation(); startEdit(ann); } }, 'Edit'),
        el('button', { className: 'btn-delete', onClick: (e) => { e.stopPropagation(); showDeleteConfirm(ann.id); } }, 'Delete'),
      ),
      deleteConfirm,
    ),
  );

  const card = el('div', { className: 'annotation-card', dataset: { id: ann.id } }, summary, detail);
  return card;
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
    } else {
      detail.classList.remove('open');
      chevron.classList.remove('expanded');
    }
  });
}

// --- Edit ---

function startEdit(ann) {
  editingId = ann.id;
  const wrap = document.getElementById(`comment-wrap-${ann.id}`);
  wrap.textContent = '';

  const textarea = el('textarea', { className: 'edit-textarea' }, getComment(ann));
  const actions = el('div', { className: 'edit-actions' },
    el('button', { className: 'btn-save-edit', onClick: () => saveEdit(ann) }, 'Save'),
    el('button', { className: 'btn-cancel-edit', onClick: () => cancelEdit(ann) }, 'Cancel'),
  );

  wrap.appendChild(textarea);
  wrap.appendChild(actions);
  textarea.focus();
}

function cancelEdit(ann) {
  editingId = null;
  const wrap = document.getElementById(`comment-wrap-${ann.id}`);
  wrap.textContent = '';
  wrap.appendChild(el('p', { className: 'detail-comment' }, getComment(ann)));
}

function saveEdit(ann) {
  const wrap = document.getElementById(`comment-wrap-${ann.id}`);
  const textarea = wrap.querySelector('.edit-textarea');
  const newValue = textarea.value.trim();

  if (!newValue) return;

  const updatedBody = (ann.annotation.body || []).map((b) => {
    if (b.type === 'TextualBody' && b.purpose === 'commenting') {
      return { ...b, value: newValue };
    }
    return b;
  });

  chrome.runtime.sendMessage({
    type: 'update-annotation',
    data: { id: ann.id, annotation: { body: updatedBody } },
  }, (response) => {
    editingId = null;
    if (chrome.runtime.lastError || !response?.ok) {
      const wrap = document.getElementById(`comment-wrap-${ann.id}`);
      if (wrap) {
        const err = el('p', { className: 'edit-error' }, 'Failed to save. Try again.');
        wrap.appendChild(err);
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

  filterBar.querySelectorAll('.filter-btn').forEach((b) => b.classList.remove('active'));
  btn.classList.add('active');
  currentFilter = btn.dataset.state || '';
  fetchAnnotations();
});

// --- Auto-refresh ---

chrome.runtime.onMessage.addListener((message) => {
  if (message.type === 'annotation-created' && message.data) {
    if (!currentFilter || message.data.state === currentFilter) {
      annotations.unshift(message.data);
      renderList();
    }
  }
});

// --- Init ---

chrome.runtime.sendMessage({ type: 'get-server-url' }, (response) => {
  if (chrome.runtime.lastError) return;
  if (response?.url) {
    serverUrl = response.url;
    serverUrlInput.value = response.url;
  }
  checkHealth();
  fetchAnnotations();
});

setInterval(checkHealth, 30000);
