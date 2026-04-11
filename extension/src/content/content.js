(function () {
  'use strict';

  if (window.__annPluginLoaded) return;
  window.__annPluginLoaded = true;

  let state = 'idle';
  let container = null;
  let cropper = null;
  let screenshotImg = null;
  let captureData = null;
  let fabricCanvas = null;
  let activeTool = 'rect';
  let activeColor = '#FF0000';
  let isDrawing = false;
  let drawStart = null;
  let drawShape = null;
  let toolbar = null;
  let hasMarkup = false;

  const COLORS = [
    { name: 'Red', value: '#FF0000' },
    { name: 'Blue', value: '#2563EB' },
    { name: 'Green', value: '#16A34A' },
    { name: 'Yellow', value: '#EAB308' },
    { name: 'White', value: '#FFFFFF' },
    { name: 'Black', value: '#000000' }
  ];

  const TOOL_DEFS = [
    { id: 'rect', label: 'Rectangle', svgPaths: [{ tag: 'rect', attrs: { x: 3, y: 3, width: 12, height: 12, rx: 1 } }] },
    { id: 'arrow', label: 'Arrow', svgPaths: [
      { tag: 'line', attrs: { x1: 4, y1: 14, x2: 14, y2: 4 } },
      { tag: 'polyline', attrs: { points: '8,4 14,4 14,10' } }
    ] },
    { id: 'highlight', label: 'Highlight', svgPaths: [
      { tag: 'path', attrs: { d: 'M3 14l4-4 3 3 4-4 4 4' } },
      { tag: 'line', attrs: { x1: 3, y1: 17, x2: 21, y2: 17, 'stroke-width': 3, opacity: 0.4 } }
    ] },
    { id: 'text', label: 'Text', svgPaths: [
      { tag: 'text', attrs: { x: 9, y: 14, 'font-size': 14, 'font-weight': 'bold', fill: 'currentColor', stroke: 'none', 'text-anchor': 'middle' }, text: 'T' }
    ] }
  ];

  chrome.runtime.onMessage.addListener((message, _sender, sendResponse) => {
    if (message.type === 'ping') {
      sendResponse({ ok: true });
      return;
    }
    if (message.type === 'start-capture' && state === 'idle') {
      startCapture();
    }
  });

  document.addEventListener('keydown', (e) => {
    if (e.key === 'Escape' && state !== 'idle') {
      cancelCapture();
    }
    if ((e.ctrlKey || e.metaKey) && e.key === 'z' && state === 'markup' && fabricCanvas) {
      e.preventDefault();
      undoLast();
    }
  });

  function setState(newState) {
    state = newState;
  }

  function createSvgIcon(paths) {
    const svg = document.createElementNS('http://www.w3.org/2000/svg', 'svg');
    svg.setAttribute('viewBox', '0 0 18 18');
    paths.forEach(function (p) {
      const el = document.createElementNS('http://www.w3.org/2000/svg', p.tag);
      Object.keys(p.attrs).forEach(function (k) { el.setAttribute(k, p.attrs[k]); });
      if (p.text) el.textContent = p.text;
      svg.appendChild(el);
    });
    return svg;
  }

  // --- Phase: Capture ---

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

  // --- Phase: Region Selection (Cropper.js) ---

  function initRegionSelection(dataUrl) {
    container = document.createElement('div');
    container.className = 'ann-overlay-container';

    screenshotImg = document.createElement('img');
    screenshotImg.src = dataUrl;
    screenshotImg.className = 'ann-screenshot-img';
    screenshotImg.style.width = '100vw';
    screenshotImg.style.height = '100vh';
    screenshotImg.style.objectFit = 'cover';
    screenshotImg.style.display = 'block';

    container.appendChild(screenshotImg);
    document.documentElement.appendChild(container);

    setTimeout(() => {
      cropper = new Cropper(screenshotImg, {
        viewMode: 1,
        dragMode: 'crop',
        autoCrop: false,
        cropBoxMovable: true,
        cropBoxResizable: true,
        background: false,
        modal: true,
        guides: false,
        center: false,
        highlight: true,
        toggleDragModeOnDblclick: false,
        ready: function () {
          setState('selected');
          showCropActions();
        },
        crop: function () {
          updateCropActions();
        }
      });
    }, 50);
  }

  function showCropActions() {
    const actions = document.createElement('div');
    actions.className = 'ann-crop-actions';
    actions.id = 'ann-crop-actions';

    const confirmBtn = document.createElement('button');
    confirmBtn.className = 'ann-btn ann-btn-confirm';
    confirmBtn.textContent = 'Confirm';
    confirmBtn.addEventListener('click', confirmSelection);

    const cancelBtn = document.createElement('button');
    cancelBtn.className = 'ann-btn ann-btn-cancel';
    cancelBtn.textContent = 'Cancel';
    cancelBtn.addEventListener('click', cancelCapture);

    actions.appendChild(confirmBtn);
    actions.appendChild(cancelBtn);
    container.appendChild(actions);
  }

  function updateCropActions() {
    const actions = document.getElementById('ann-crop-actions');
    if (!actions || !cropper) return;

    const cropBox = cropper.getCropBoxData();
    if (!cropBox || cropBox.width === 0) return;

    const top = cropBox.top - 44;
    var actionsW = actions.offsetWidth || 200;
    actions.style.left = Math.min(cropBox.left, window.innerWidth - actionsW - 8) + 'px';
    actions.style.top = (top > 0 ? top : cropBox.top + cropBox.height + 8) + 'px';
  }

  function confirmSelection() {
    if (!cropper) return;

    const data = cropper.getData(true);
    if (data.width < 5 || data.height < 5) return;

    const dpr = window.devicePixelRatio || 1;
    const croppedCanvas = cropper.getCroppedCanvas({
      imageSmoothingEnabled: true,
      imageSmoothingQuality: 'high'
    });

    captureData = {
      regionX: Math.round(data.x / dpr),
      regionY: Math.round(data.y / dpr),
      regionW: Math.round(data.width / dpr),
      regionH: Math.round(data.height / dpr),
      croppedCanvas: croppedCanvas,
      viewportWidth: window.innerWidth,
      viewportHeight: window.innerHeight,
      pageUrl: window.location.href,
      dpr: dpr
    };

    cropper.destroy();
    cropper = null;
    screenshotImg.remove();
    screenshotImg = null;

    const actionsEl = document.getElementById('ann-crop-actions');
    if (actionsEl) actionsEl.remove();

    initMarkup(captureData);
  }

  // --- Phase: Markup (Fabric.js) ---

  function initMarkup(data) {
    setState('markup');
    hasMarkup = false;
    activeTool = 'rect';
    activeColor = '#FF0000';

    var scrim = document.createElement('div');
    scrim.className = 'ann-markup-scrim';
    container.appendChild(scrim);

    var frame = document.createElement('div');
    frame.className = 'ann-markup-frame';
    frame.style.left = (data.regionX - 2) + 'px';
    frame.style.top = (data.regionY - 2) + 'px';
    frame.style.width = (data.regionW + 4) + 'px';
    frame.style.height = (data.regionH + 4) + 'px';
    container.appendChild(frame);

    const dpr = data.dpr;
    const canvasEl = document.createElement('canvas');
    canvasEl.id = 'ann-fabric-canvas';
    canvasEl.width = data.regionW;
    canvasEl.height = data.regionH;

    container.appendChild(canvasEl);

    fabricCanvas = new fabric.Canvas(canvasEl, {
      selection: false,
      renderOnAddRemove: true
    });

    var wrapper = fabricCanvas.wrapperEl;
    wrapper.style.position = 'fixed';
    wrapper.style.left = data.regionX + 'px';
    wrapper.style.top = data.regionY + 'px';
    wrapper.style.zIndex = '2147483643';

    const bgImg = new Image();
    bgImg.onload = function () {
      const fabricBg = new fabric.FabricImage(bgImg, {
        scaleX: data.regionW / bgImg.width,
        scaleY: data.regionH / bgImg.height,
        selectable: false,
        evented: false
      });
      fabricCanvas.backgroundImage = fabricBg;
      fabricCanvas.renderAll();
    };
    bgImg.src = data.croppedCanvas.toDataURL('image/png');

    setupDrawingHandlers(1);
    showToolbar(data);
  }

  function showToolbar(data) {
    toolbar = document.createElement('div');
    toolbar.className = 'ann-toolbar';

    const toolbarTop = data.regionY - 48;
    toolbar.style.top = (toolbarTop > 0 ? toolbarTop : data.regionY + data.regionH + 8) + 'px';
    toolbar.style.position = 'fixed';

    TOOL_DEFS.forEach(function (tool) {
      const btn = document.createElement('button');
      btn.className = 'ann-toolbar-btn' + (tool.id === activeTool ? ' active' : '');
      btn.title = tool.label;
      btn.dataset.tool = tool.id;
      btn.appendChild(createSvgIcon(tool.svgPaths));
      btn.addEventListener('click', function () { selectTool(tool.id); });
      toolbar.appendChild(btn);
    });

    var divider1 = document.createElement('div');
    divider1.className = 'ann-toolbar-divider';
    toolbar.appendChild(divider1);

    var swatches = document.createElement('div');
    swatches.className = 'ann-color-swatches';
    COLORS.forEach(function (c) {
      var swatch = document.createElement('button');
      swatch.className = 'ann-color-swatch' + (c.value === activeColor ? ' active' : '');
      swatch.style.background = c.value;
      if (c.value === '#FFFFFF') swatch.style.boxShadow = 'inset 0 0 0 1px rgba(0,0,0,0.15)';
      swatch.title = c.name;
      swatch.dataset.color = c.value;
      swatch.addEventListener('click', function () { selectColor(c.value); });
      swatches.appendChild(swatch);
    });
    toolbar.appendChild(swatches);

    var divider2 = document.createElement('div');
    divider2.className = 'ann-toolbar-divider';
    toolbar.appendChild(divider2);

    var undoBtn = document.createElement('button');
    undoBtn.className = 'ann-toolbar-btn';
    undoBtn.title = 'Undo (Ctrl+Z)';
    undoBtn.appendChild(createSvgIcon([
      { tag: 'path', attrs: { d: 'M4 8l3-3M4 8l3 3M4 8h8a3 3 0 0 1 0 6H9' } }
    ]));
    undoBtn.addEventListener('click', undoLast);
    toolbar.appendChild(undoBtn);

    var divider3 = document.createElement('div');
    divider3.className = 'ann-toolbar-divider';
    toolbar.appendChild(divider3);

    var doneBtn = document.createElement('button');
    doneBtn.className = 'ann-btn ann-btn-confirm';
    doneBtn.textContent = 'Done';
    doneBtn.style.fontSize = '12px';
    doneBtn.style.padding = '4px 12px';
    doneBtn.addEventListener('click', finishMarkup);
    toolbar.appendChild(doneBtn);

    var cancelBtn = document.createElement('button');
    cancelBtn.className = 'ann-btn ann-btn-cancel';
    cancelBtn.textContent = 'Cancel';
    cancelBtn.style.fontSize = '12px';
    cancelBtn.style.padding = '4px 12px';
    cancelBtn.addEventListener('click', cancelCapture);
    toolbar.appendChild(cancelBtn);

    container.appendChild(toolbar);
    var toolbarW = toolbar.offsetWidth;
    toolbar.style.left = Math.min(data.regionX, window.innerWidth - toolbarW - 8) + 'px';
  }

  function selectTool(toolId) {
    activeTool = toolId;
    if (fabricCanvas) {
      fabricCanvas.isDrawingMode = (toolId === 'highlight');
      if (toolId === 'highlight') {
        var dpr = captureData?.dpr || 1;
        var brush = new fabric.PencilBrush(fabricCanvas);
        brush.color = hexToRgba(activeColor, 0.3);
        brush.width = 12 * dpr;
        fabricCanvas.freeDrawingBrush = brush;
      }
    }
    toolbar.querySelectorAll('.ann-toolbar-btn[data-tool]').forEach(function (btn) {
      btn.classList.toggle('active', btn.dataset.tool === toolId);
    });
  }

  function selectColor(color) {
    activeColor = color;
    toolbar.querySelectorAll('.ann-color-swatch').forEach(function (s) {
      s.classList.toggle('active', s.dataset.color === color);
    });
    if (fabricCanvas && fabricCanvas.isDrawingMode && fabricCanvas.freeDrawingBrush) {
      fabricCanvas.freeDrawingBrush.color = hexToRgba(color, 0.3);
    }
  }

  function hexToRgba(hex, alpha) {
    var r = parseInt(hex.slice(1, 3), 16);
    var g = parseInt(hex.slice(3, 5), 16);
    var b = parseInt(hex.slice(5, 7), 16);
    return 'rgba(' + r + ',' + g + ',' + b + ',' + alpha + ')';
  }

  function undoLast() {
    if (!fabricCanvas) return;
    var objects = fabricCanvas.getObjects();
    if (objects.length === 0) return;
    fabricCanvas.remove(objects[objects.length - 1]);
    fabricCanvas.renderAll();
    hasMarkup = fabricCanvas.getObjects().length > 0;
  }

  function setupDrawingHandlers(dpr) {
    fabricCanvas.on('mouse:down', function (opt) {
      if (fabricCanvas.isDrawingMode) { hasMarkup = true; return; }
      if (activeTool === 'text') {
        placeText(opt.pointer, dpr);
        return;
      }
      if (activeTool !== 'rect' && activeTool !== 'arrow') return;

      isDrawing = true;
      drawStart = { x: opt.pointer.x, y: opt.pointer.y };

      if (activeTool === 'rect') {
        drawShape = new fabric.Rect({
          left: drawStart.x,
          top: drawStart.y,
          width: 0,
          height: 0,
          fill: 'transparent',
          stroke: activeColor,
          strokeWidth: 2 * dpr,
          selectable: false,
          evented: false
        });
        fabricCanvas.add(drawShape);
      } else if (activeTool === 'arrow') {
        drawShape = new fabric.Line(
          [drawStart.x, drawStart.y, drawStart.x, drawStart.y],
          {
            stroke: activeColor,
            strokeWidth: 2 * dpr,
            selectable: false,
            evented: false
          }
        );
        fabricCanvas.add(drawShape);
      }
    });

    fabricCanvas.on('mouse:move', function (opt) {
      if (!isDrawing || !drawShape) return;
      var pointer = opt.pointer;

      if (activeTool === 'rect') {
        var left = Math.min(drawStart.x, pointer.x);
        var top = Math.min(drawStart.y, pointer.y);
        drawShape.set({
          left: left,
          top: top,
          width: Math.abs(pointer.x - drawStart.x),
          height: Math.abs(pointer.y - drawStart.y)
        });
      } else if (activeTool === 'arrow') {
        drawShape.set({ x2: pointer.x, y2: pointer.y });
      }
      fabricCanvas.renderAll();
    });

    fabricCanvas.on('mouse:up', function () {
      if (!isDrawing) return;
      isDrawing = false;

      if (activeTool === 'arrow' && drawShape) {
        var x1 = drawShape.x1, y1 = drawShape.y1;
        var x2 = drawShape.x2, y2 = drawShape.y2;
        var dx = x2 - x1, dy = y2 - y1;
        var len = Math.sqrt(dx * dx + dy * dy);

        if (len < 5) {
          fabricCanvas.remove(drawShape);
          drawShape = null;
          return;
        }

        fabricCanvas.remove(drawShape);

        var angle = Math.atan2(dy, dx);
        var headLen = 10 * (captureData?.dpr || 1);
        var arrowHead = new fabric.Polygon(
          [
            { x: x2, y: y2 },
            { x: x2 - headLen * Math.cos(angle - Math.PI / 6), y: y2 - headLen * Math.sin(angle - Math.PI / 6) },
            { x: x2 - headLen * Math.cos(angle + Math.PI / 6), y: y2 - headLen * Math.sin(angle + Math.PI / 6) }
          ],
          { fill: activeColor, selectable: false, evented: false }
        );

        var line = new fabric.Line([x1, y1, x2, y2], {
          stroke: activeColor,
          strokeWidth: 2 * (captureData?.dpr || 1),
          selectable: false,
          evented: false
        });

        var group = new fabric.Group([line, arrowHead], {
          selectable: false,
          evented: false
        });
        fabricCanvas.add(group);
        hasMarkup = true;
      } else if (activeTool === 'rect' && drawShape) {
        if (drawShape.width < 3 && drawShape.height < 3) {
          fabricCanvas.remove(drawShape);
        } else {
          hasMarkup = true;
        }
      }

      drawShape = null;
    });

    fabricCanvas.on('path:created', function () {
      hasMarkup = true;
    });
  }

  function placeText(pointer, dpr) {
    var text = new fabric.IText('', {
      left: pointer.x,
      top: pointer.y,
      fill: activeColor,
      fontSize: 16 * dpr,
      fontFamily: '-apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif',
      selectable: true,
      editable: true
    });

    fabricCanvas.add(text);
    fabricCanvas.setActiveObject(text);
    text.enterEditing();

    text.on('editing:exited', function () {
      if (!text.text || text.text.trim() === '') {
        fabricCanvas.remove(text);
      } else {
        text.set({ selectable: false, editable: false, evented: false });
        hasMarkup = true;
      }
      fabricCanvas.renderAll();
    });
  }

  async function finishMarkup() {
    if (!fabricCanvas || !captureData) return;

    fabricCanvas.discardActiveObject();
    fabricCanvas.renderAll();

    var compositeDataUrl = fabricCanvas.toDataURL({ format: 'png', multiplier: captureData.dpr });
    var markupSvg = hasMarkup ? fabricCanvas.toSVG() : null;

    var wrapperEl = fabricCanvas.wrapperEl;
    await fabricCanvas.dispose();
    fabricCanvas = null;

    if (toolbar) { toolbar.remove(); toolbar = null; }
    if (wrapperEl && wrapperEl.parentElement) wrapperEl.remove();

    captureData.compositeDataUrl = compositeDataUrl;
    captureData.hasMarkup = hasMarkup;
    captureData.markupSvg = markupSvg;

    showCommentInput(captureData);
  }

  // --- Phase: Comment Input ---

  function showCommentInput(data) {
    setState('commenting');

    var panel = document.createElement('div');
    panel.className = 'ann-comment-panel';
    panel.style.left = Math.min(data.regionX, window.innerWidth - 360) + 'px';
    panel.style.top = Math.min(data.regionY, window.innerHeight - 340) + 'px';

    var thumb = document.createElement('img');
    thumb.className = 'ann-comment-thumbnail';
    thumb.src = data.compositeDataUrl;
    panel.appendChild(thumb);

    var textarea = document.createElement('textarea');
    textarea.className = 'ann-comment-textarea';
    textarea.placeholder = 'Add a comment...';
    panel.appendChild(textarea);

    var actions = document.createElement('div');
    actions.className = 'ann-comment-actions';

    var cancelBtn = document.createElement('button');
    cancelBtn.className = 'ann-btn ann-btn-cancel';
    cancelBtn.textContent = 'Cancel';
    cancelBtn.addEventListener('click', cancelCapture);

    var saveBtn = document.createElement('button');
    saveBtn.className = 'ann-btn ann-btn-confirm';
    saveBtn.textContent = 'Save';
    saveBtn.addEventListener('click', function () { submitAnnotation(textarea.value, data); });

    actions.appendChild(cancelBtn);
    actions.appendChild(saveBtn);
    panel.appendChild(actions);

    container.appendChild(panel);

    setTimeout(function () { textarea.focus(); }, 50);

    textarea.addEventListener('keydown', function (e) {
      if (e.key === 'Enter' && !e.shiftKey) {
        e.preventDefault();
        submitAnnotation(textarea.value, data);
      }
      if (e.key === 'Escape') {
        e.stopPropagation();
        cancelCapture();
      }
    });
  }

  function submitAnnotation(commentText, data) {
    var now = new Date().toISOString();
    var body = [];
    if (commentText && commentText.trim()) {
      body.push({
        type: 'TextualBody',
        value: commentText.trim(),
        purpose: 'commenting'
      });
    }
    body.push({ type: 'Image' });

    var selectors = [
      {
        type: 'FragmentSelector',
        conformsTo: 'http://www.w3.org/TR/media-frags/',
        value: 'xywh=' + data.regionX + ',' + data.regionY + ',' + data.regionW + ',' + data.regionH
      }
    ];

    if (data.hasMarkup && data.markupSvg) {
      selectors.push({
        type: 'SvgSelector',
        value: data.markupSvg
      });
    }

    var annotation = {
      '@context': 'http://www.w3.org/ns/anno.jsonld',
      type: 'Annotation',
      motivation: 'commenting',
      created: now,
      modified: now,
      creator: {
        type: 'Person',
        name: 'developer'
      },
      body: body,
      target: {
        source: data.pageUrl,
        selector: selectors,
        state: {
          type: 'HttpRequestState',
          value: 'viewport=' + data.viewportWidth + 'x' + data.viewportHeight
        }
      }
    };

    chrome.runtime.sendMessage({
      type: 'create-annotation',
      data: { annotation: annotation, imageDataUrl: data.compositeDataUrl }
    }, function (response) {
      if (response?.error) {
        showToast('Failed to save annotation');
        return;
      }
      cancelCapture();
      showToast('Annotation saved');
    });
  }

  function showToast(message) {
    var toast = document.createElement('div');
    toast.className = 'ann-toast';

    var iconSvg = document.createElementNS('http://www.w3.org/2000/svg', 'svg');
    iconSvg.setAttribute('class', 'ann-toast-icon');
    iconSvg.setAttribute('viewBox', '0 0 18 18');
    var polyline = document.createElementNS('http://www.w3.org/2000/svg', 'polyline');
    polyline.setAttribute('points', '4 9 7 12 14 5');
    iconSvg.appendChild(polyline);
    toast.appendChild(iconSvg);

    var span = document.createElement('span');
    span.textContent = message;
    toast.appendChild(span);

    document.documentElement.appendChild(toast);
    setTimeout(function () {
      toast.style.transition = 'opacity 0.3s ease';
      toast.style.opacity = '0';
      setTimeout(function () { toast.remove(); }, 300);
    }, 1500);
  }

  // --- Teardown ---

  function cancelCapture() {
    if (fabricCanvas) {
      fabricCanvas.dispose();
      fabricCanvas = null;
    }
    if (cropper) {
      cropper.destroy();
      cropper = null;
    }
    screenshotImg = null;
    captureData = null;
    isDrawing = false;
    drawShape = null;
    toolbar = null;
    hasMarkup = false;
    if (container) {
      container.remove();
      container = null;
    }
    setState('idle');
  }
})();
