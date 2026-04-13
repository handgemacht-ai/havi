(function () {
  'use strict';

  if (window.__annContextCollector) return;
  window.__annContextCollector = true;

  var bufferSize = parseInt(document.documentElement.dataset.annBufferSize, 10) || 10;

  function RingBuffer(size) {
    this.buf = [];
    this.size = size;
  }
  RingBuffer.prototype.push = function (item) {
    this.buf.push(item);
    if (this.buf.length > this.size) this.buf.shift();
  };
  RingBuffer.prototype.entries = function () {
    return this.buf.slice();
  };

  var consoleErrors = new RingBuffer(bufferSize);
  var networkErrors = new RingBuffer(bufferSize);

  // Console capture — error and warn only
  var origError = console.error;
  var origWarn = console.warn;

  console.error = function () {
    var args = Array.prototype.slice.call(arguments);
    consoleErrors.push({
      level: 'error',
      message: args.map(String).join(' '),
      timestamp: Date.now()
    });
    return origError.apply(console, args);
  };

  console.warn = function () {
    var args = Array.prototype.slice.call(arguments);
    consoleErrors.push({
      level: 'warn',
      message: args.map(String).join(' '),
      timestamp: Date.now()
    });
    return origWarn.apply(console, args);
  };

  // Fetch interception — 4xx/5xx only
  var origFetch = window.fetch;
  window.fetch = function () {
    var args = Array.prototype.slice.call(arguments);
    var input = args[0];
    var init = args[1] || {};
    var method = (init.method || 'GET').toUpperCase();
    var url = typeof input === 'string' ? input : (input && input.url ? input.url : String(input));

    return origFetch.apply(window, args).then(function (response) {
      if (response.status >= 400) {
        networkErrors.push({
          method: method,
          url: url,
          status: response.status,
          statusText: response.statusText,
          timestamp: Date.now()
        });
      }
      return response;
    });
  };

  // XHR interception — 4xx/5xx only
  var origOpen = XMLHttpRequest.prototype.open;
  var origSend = XMLHttpRequest.prototype.send;

  XMLHttpRequest.prototype.open = function (method, url) {
    this._annMethod = (method || 'GET').toUpperCase();
    this._annUrl = String(url);
    return origOpen.apply(this, arguments);
  };

  XMLHttpRequest.prototype.send = function () {
    var xhr = this;
    xhr.addEventListener('loadend', function () {
      if (xhr.status >= 400) {
        networkErrors.push({
          method: xhr._annMethod || 'GET',
          url: xhr._annUrl || '',
          status: xhr.status,
          statusText: xhr.statusText,
          timestamp: Date.now()
        });
      }
    });
    return origSend.apply(this, arguments);
  };

  // Web vitals via PerformanceObserver
  var vitals = { lcp: null, cls: null, inp: null };

  try {
    new PerformanceObserver(function (list) {
      var entries = list.getEntries();
      if (entries.length) {
        vitals.lcp = entries[entries.length - 1].startTime;
      }
    }).observe({ type: 'largest-contentful-paint', buffered: true });
  } catch (e) {}

  try {
    var clsValue = 0;
    new PerformanceObserver(function (list) {
      var entries = list.getEntries();
      for (var i = 0; i < entries.length; i++) {
        if (!entries[i].hadRecentInput) {
          clsValue += entries[i].value;
        }
      }
      vitals.cls = clsValue;
    }).observe({ type: 'layout-shift', buffered: true });
  } catch (e) {}

  try {
    var interactions = {};
    new PerformanceObserver(function (list) {
      var entries = list.getEntries();
      for (var i = 0; i < entries.length; i++) {
        var entry = entries[i];
        if (entry.interactionId) {
          var existing = interactions[entry.interactionId];
          if (!existing || entry.duration > existing) {
            interactions[entry.interactionId] = entry.duration;
          }
        }
      }
      var durations = Object.keys(interactions).map(function (k) { return interactions[k]; });
      if (durations.length) {
        durations.sort(function (a, b) { return a - b; });
        var p98Index = Math.max(0, Math.ceil(durations.length * 0.98) - 1);
        vitals.inp = durations[p98Index];
      }
    }).observe({ type: 'event', buffered: true, durationThreshold: 16 });
  } catch (e) {}

  // CustomEvent bridge
  window.addEventListener('__ann_request_context', function () {
    var appContext = null;
    try {
      var hook = window.__annotationContext;
      if (typeof hook === 'function') {
        appContext = hook();
      } else if (hook && typeof hook === 'object') {
        appContext = hook;
      }
    } catch (e) {}

    window.dispatchEvent(new CustomEvent('__ann_context_response', {
      detail: {
        appContext: appContext,
        consoleErrors: consoleErrors.entries(),
        networkErrors: networkErrors.entries(),
        webVitals: {
          lcp: vitals.lcp,
          cls: vitals.cls,
          inp: vitals.inp
        }
      }
    }));
  });
})();
