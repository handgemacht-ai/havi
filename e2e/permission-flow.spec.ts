import { test, expect } from './fixtures';

declare global {
  interface Window {
    __haviCalls: { kind: string; message?: unknown; permissions?: unknown }[];
    __haviResponder: (msg: { type?: string }) => unknown;
    __haviPermissionsResponder: (perms: { origins?: string[] }) => boolean;
  }
}

async function installSidePanelStubs(sidePanel: import('@playwright/test').Page) {
  await sidePanel.addInitScript(() => {
    const calls: { kind: string; message?: unknown; permissions?: unknown }[] = [];
    window.__haviCalls = calls;

    window.__haviResponder = (msg) => {
      switch (msg?.type) {
        case 'get-server-url':
          return { url: 'http://localhost:8090' };
        case 'check-health':
          return { ok: true };
        case 'list-annotations':
          return { ok: true, data: [], meta: { count: 0 } };
        default:
          return { ok: true };
      }
    };
    window.__haviPermissionsResponder = () => true;

    const install = () => {
      const c = (globalThis as { chrome?: typeof chrome }).chrome;
      if (!c?.runtime?.sendMessage || !c?.permissions?.request) {
        requestAnimationFrame(install);
        return;
      }
      (c.runtime as unknown as { sendMessage: (...args: unknown[]) => unknown }).sendMessage = function (
        message: { type?: string },
        callback?: (response: unknown) => void,
      ) {
        calls.push({ kind: 'sendMessage', message });
        const response = window.__haviResponder(message);
        if (typeof callback === 'function') {
          queueMicrotask(() => callback(response));
          return undefined;
        }
        return Promise.resolve(response);
      };
      (c.permissions as unknown as { request: (...args: unknown[]) => unknown }).request = function (
        permissions: { origins?: string[] },
        callback?: (granted: boolean) => void,
      ) {
        calls.push({ kind: 'permissions.request', permissions });
        const granted = window.__haviPermissionsResponder(permissions);
        if (typeof callback === 'function') {
          queueMicrotask(() => callback(granted));
          return undefined;
        }
        return Promise.resolve(granted);
      };
    };
    install();
  });
}

test('classifyCaptureFailure returns the shape the side panel relies on', async ({ serviceWorker }) => {
  const cases = await serviceWorker.evaluate(() => {
    const fn = (globalThis as { classifyCaptureFailure?: (e: unknown, t: unknown) => unknown })
      .classifyCaptureFailure;
    if (!fn) return { error: 'classifyCaptureFailure not on globalThis' };

    const permError = new Error("Either the '<all_urls>' or 'activeTab' permission is required.");
    return {
      permWithUrl: fn(permError, { url: 'https://example.com/page' }),
      permWithoutUrl: fn(permError, { url: undefined }),
      chromeUrl: fn(new Error('Cannot capture this page'), { url: 'chrome://newtab/' }),
      otherWithUrl: fn(new Error('Server unreachable'), { url: 'https://example.com/' }),
    };
  });

  expect(cases).toMatchObject({
    permWithUrl: { ok: false, code: 'permission_required', origin: 'https://example.com' },
    permWithoutUrl: { ok: false, code: 'permission_required', origin: null },
    chromeUrl: { ok: false, code: 'unsupported_page', origin: null },
    otherWithUrl: { ok: false, code: 'other', origin: 'https://example.com' },
  });
});

test('side panel renders permission_required alert with a Grant access button', async ({
  context,
  serviceWorker,
}) => {
  const extensionId = new URL(serviceWorker.url()).host;
  const sidePanel = await context.newPage();
  await installSidePanelStubs(sidePanel);

  await sidePanel.goto(`chrome-extension://${extensionId}/src/sidepanel/sidepanel.html`);
  await sidePanel.waitForFunction(() => typeof chrome !== 'undefined' && !!chrome.runtime?.id);

  await sidePanel.evaluate(() => {
    window.__haviResponder = (msg) => {
      if (msg?.type === 'start-capture-from-panel') {
        return {
          ok: false,
          error: 'simulated permission failure',
          code: 'permission_required',
          origin: 'https://example.com',
        };
      }
      if (msg?.type === 'get-server-url') return { url: 'http://localhost:8090' };
      if (msg?.type === 'check-health') return { ok: true };
      if (msg?.type === 'list-annotations') return { ok: true, data: [], meta: { count: 0 } };
      return { ok: true };
    };
  });

  await sidePanel.locator('#capture-btn').click();

  const alert = sidePanel.locator('#capture-alert');
  await expect(alert).toBeVisible();
  await expect(sidePanel.locator('#capture-alert-message')).toContainText('take a screenshot');
  await expect(sidePanel.locator('#capture-alert-action')).toBeVisible();
  await expect(sidePanel.locator('#capture-alert-action')).toHaveText('Grant access');

  await expect(sidePanel.locator('#capture-btn')).toContainText('Capture');
  await expect(sidePanel.locator('#capture-btn')).not.toHaveClass(/cancel/);

  await sidePanel.close();
});

test('side panel renders unsupported_page alert without a Grant access button', async ({
  context,
  serviceWorker,
}) => {
  const extensionId = new URL(serviceWorker.url()).host;
  const sidePanel = await context.newPage();
  await installSidePanelStubs(sidePanel);

  await sidePanel.goto(`chrome-extension://${extensionId}/src/sidepanel/sidepanel.html`);
  await sidePanel.waitForFunction(() => typeof chrome !== 'undefined' && !!chrome.runtime?.id);

  await sidePanel.evaluate(() => {
    window.__haviResponder = (msg) => {
      if (msg?.type === 'start-capture-from-panel') {
        return { ok: false, error: 'Cannot capture this page', code: 'unsupported_page', origin: null };
      }
      if (msg?.type === 'get-server-url') return { url: 'http://localhost:8090' };
      if (msg?.type === 'check-health') return { ok: true };
      if (msg?.type === 'list-annotations') return { ok: true, data: [], meta: { count: 0 } };
      return { ok: true };
    };
  });

  await sidePanel.locator('#capture-btn').click();

  await expect(sidePanel.locator('#capture-alert')).toBeVisible();
  await expect(sidePanel.locator('#capture-alert-message')).toContainText('not available on this page');
  await expect(sidePanel.locator('#capture-alert-action')).toBeHidden();

  await sidePanel.close();
});

test('Grant access calls chrome.permissions.request and retries the capture', async ({
  context,
  serviceWorker,
}) => {
  const extensionId = new URL(serviceWorker.url()).host;
  const sidePanel = await context.newPage();
  await installSidePanelStubs(sidePanel);

  await sidePanel.goto(`chrome-extension://${extensionId}/src/sidepanel/sidepanel.html`);
  await sidePanel.waitForFunction(() => typeof chrome !== 'undefined' && !!chrome.runtime?.id);

  await sidePanel.evaluate(() => {
    let captureAttempts = 0;
    window.__haviResponder = (msg) => {
      if (msg?.type === 'start-capture-from-panel') {
        captureAttempts++;
        if (captureAttempts === 1) {
          return {
            ok: false,
            error: 'simulated',
            code: 'permission_required',
            origin: 'https://example.com',
          };
        }
        return { ok: true };
      }
      if (msg?.type === 'get-server-url') return { url: 'http://localhost:8090' };
      if (msg?.type === 'check-health') return { ok: true };
      if (msg?.type === 'list-annotations') return { ok: true, data: [], meta: { count: 0 } };
      return { ok: true };
    };
    window.__haviPermissionsResponder = () => true;
  });

  await sidePanel.locator('#capture-btn').click();
  await expect(sidePanel.locator('#capture-alert-action')).toHaveText('Grant access');

  await sidePanel.locator('#capture-alert-action').click();

  await expect(sidePanel.locator('#capture-alert')).toBeHidden();

  const recorded = await sidePanel.evaluate(() => window.__haviCalls);
  const permissionRequests = recorded.filter((c) => c.kind === 'permissions.request');
  expect(permissionRequests).toHaveLength(1);
  expect(permissionRequests[0].permissions).toEqual({ origins: ['https://*/*', 'http://*/*'] });

  const captureMessages = recorded.filter(
    (c) => c.kind === 'sendMessage' && (c.message as { type?: string })?.type === 'start-capture-from-panel',
  );
  expect(captureMessages).toHaveLength(2);

  await expect(sidePanel.locator('#capture-btn')).toContainText('Cancel capture');

  await sidePanel.close();
});

test('denied permission grant leaves the alert visible with a denial message', async ({
  context,
  serviceWorker,
}) => {
  const extensionId = new URL(serviceWorker.url()).host;
  const sidePanel = await context.newPage();
  await installSidePanelStubs(sidePanel);

  await sidePanel.goto(`chrome-extension://${extensionId}/src/sidepanel/sidepanel.html`);
  await sidePanel.waitForFunction(() => typeof chrome !== 'undefined' && !!chrome.runtime?.id);

  await sidePanel.evaluate(() => {
    window.__haviResponder = (msg) => {
      if (msg?.type === 'start-capture-from-panel') {
        return {
          ok: false,
          error: 'simulated',
          code: 'permission_required',
          origin: 'https://example.com',
        };
      }
      if (msg?.type === 'get-server-url') return { url: 'http://localhost:8090' };
      if (msg?.type === 'check-health') return { ok: true };
      if (msg?.type === 'list-annotations') return { ok: true, data: [], meta: { count: 0 } };
      return { ok: true };
    };
    window.__haviPermissionsResponder = () => false;
  });

  await sidePanel.locator('#capture-btn').click();
  await sidePanel.locator('#capture-alert-action').click();

  await expect(sidePanel.locator('#capture-alert')).toBeVisible();
  await expect(sidePanel.locator('#capture-alert-message')).toContainText('not granted');

  await sidePanel.close();
});
