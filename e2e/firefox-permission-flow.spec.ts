import { test, expect, expectLocator, type MarionettePage } from './firefox-fixtures';

const SIDEPANEL_PATH = '/src/sidepanel/sidepanel.html';
const SIDEPANEL_URL = `moz-extension://00000000-0000-0000-0000-0000000000a1${SIDEPANEL_PATH}`;

async function openSidePanelWithStubs(
  newExtensionPage: () => Promise<MarionettePage>,
): Promise<MarionettePage> {
  const sidePanel = await newExtensionPage();
  await sidePanel.addInitScript(() => {
    const w = window as unknown as Record<string, unknown>;
    const calls: { kind: string; message?: unknown; permissions?: unknown }[] = [];
    w.__haviCalls = calls;

    w.__haviResponder = (msg: { type?: string }) => {
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
    w.__haviPermissionsResponder = () => true;

    const install = () => {
      const c = chrome as unknown as {
        runtime?: { sendMessage?: (...args: unknown[]) => unknown };
        permissions?: { request?: (...args: unknown[]) => unknown };
      };
      if (!c?.runtime?.sendMessage || !c?.permissions?.request) {
        requestAnimationFrame(install);
        return;
      }
      (c.runtime as unknown as { sendMessage: (...args: unknown[]) => unknown }).sendMessage =
        function (message: { type?: string }, callback?: (response: unknown) => void) {
          (w.__haviCalls as { kind: string; message?: unknown }[]).push({ kind: 'sendMessage', message });
          const response = (w.__haviResponder as (m: unknown) => unknown)(message);
          if (typeof callback === 'function') {
            queueMicrotask(() => callback(response));
            return undefined;
          }
          return Promise.resolve(response);
        };
      (c.permissions as unknown as { request: (...args: unknown[]) => unknown }).request =
        function (permissions: { origins?: string[] }, callback?: (granted: boolean) => void) {
          (w.__haviCalls as { kind: string; permissions?: unknown }[]).push({ kind: 'permissions.request', permissions });
          const granted = (w.__haviPermissionsResponder as (p: unknown) => boolean)(permissions);
          if (typeof callback === 'function') {
            queueMicrotask(() => callback(granted));
            return undefined;
          }
          return Promise.resolve(granted);
        };
    };
    install();
  });
  await sidePanel.goto(SIDEPANEL_URL);
  await sidePanel.waitForFunction(() => typeof chrome !== 'undefined' && !!chrome.runtime?.id);
  return sidePanel;
}

test('side panel renders permission_required alert with a Grant access button (firefox)', async ({
  newExtensionPage,
}) => {
  const sidePanel = await openSidePanelWithStubs(newExtensionPage);

  await sidePanel.evaluate(() => {
    const w = window as unknown as Record<string, unknown>;
    w.__haviResponder = (msg: { type?: string }) => {
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

  await expectLocator(sidePanel.locator('#capture-alert')).toBeVisible();
  await expectLocator(sidePanel.locator('#capture-alert-message')).toContainText('take a screenshot');
  await expectLocator(sidePanel.locator('#capture-alert-action')).toBeVisible();
  await expectLocator(sidePanel.locator('#capture-alert-action')).toHaveText('Grant access');

  await expectLocator(sidePanel.locator('#capture-btn')).toContainText('Capture');
  await expectLocator(sidePanel.locator('#capture-btn')).not.toHaveClass(/cancel/);

  await sidePanel.close();
});

test('side panel renders unsupported_page alert without a Grant access button (firefox)', async ({
  newExtensionPage,
}) => {
  const sidePanel = await openSidePanelWithStubs(newExtensionPage);

  await sidePanel.evaluate(() => {
    const w = window as unknown as Record<string, unknown>;
    w.__haviResponder = (msg: { type?: string }) => {
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

  await expectLocator(sidePanel.locator('#capture-alert')).toBeVisible();
  await expectLocator(sidePanel.locator('#capture-alert-message')).toContainText('not available on this page');
  await expectLocator(sidePanel.locator('#capture-alert-action')).toBeHidden();

  await sidePanel.close();
});

test('Grant access calls chrome.permissions.request and retries the capture (firefox)', async ({
  newExtensionPage,
}) => {
  const sidePanel = await openSidePanelWithStubs(newExtensionPage);

  await sidePanel.evaluate(() => {
    const w = window as unknown as Record<string, unknown>;
    let captureAttempts = 0;
    w.__haviResponder = (msg: { type?: string }) => {
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
    w.__haviPermissionsResponder = () => true;
  });

  await sidePanel.locator('#capture-btn').click();
  await expectLocator(sidePanel.locator('#capture-alert-action')).toHaveText('Grant access');

  await sidePanel.locator('#capture-alert-action').click();

  await expectLocator(sidePanel.locator('#capture-alert')).toBeHidden();

  const recorded = await sidePanel.evaluate(
    () => (window as unknown as { __haviCalls: unknown }).__haviCalls,
  );
  const calls = recorded as { kind: string; message?: { type?: string }; permissions?: unknown }[];
  const permissionRequests = calls.filter((c) => c.kind === 'permissions.request');
  expect(permissionRequests).toHaveLength(1);
  expect(permissionRequests[0].permissions).toEqual({ origins: ['<all_urls>'] });

  const captureMessages = calls.filter(
    (c) => c.kind === 'sendMessage' && c.message?.type === 'start-capture-from-panel',
  );
  expect(captureMessages).toHaveLength(2);

  await expectLocator(sidePanel.locator('#capture-btn')).toContainText('Cancel capture');

  await sidePanel.close();
});

test('Pick element retries with start-pick-element-from-panel after Grant access (firefox)', async ({
  newExtensionPage,
}) => {
  const sidePanel = await openSidePanelWithStubs(newExtensionPage);

  await sidePanel.evaluate(() => {
    const w = window as unknown as Record<string, unknown>;
    let attempts = 0;
    w.__haviResponder = (msg: { type?: string }) => {
      if (msg?.type === 'start-pick-element-from-panel') {
        attempts++;
        if (attempts === 1) {
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
    w.__haviPermissionsResponder = () => true;
  });

  await sidePanel.locator('#pick-element-btn').click();
  await expectLocator(sidePanel.locator('#capture-alert-action')).toHaveText('Grant access');
  await sidePanel.locator('#capture-alert-action').click();

  await expectLocator(sidePanel.locator('#capture-alert')).toBeHidden();

  const recorded = await sidePanel.evaluate(
    () => (window as unknown as { __haviCalls: unknown }).__haviCalls,
  );
  const calls = recorded as { kind: string; message?: { type?: string }; permissions?: unknown }[];
  const permissionRequests = calls.filter((c) => c.kind === 'permissions.request');
  expect(permissionRequests).toHaveLength(1);
  expect(permissionRequests[0].permissions).toEqual({ origins: ['<all_urls>'] });

  const pickMessages = calls.filter(
    (c) => c.kind === 'sendMessage' && c.message?.type === 'start-pick-element-from-panel',
  );
  expect(pickMessages).toHaveLength(2);

  await expectLocator(sidePanel.locator('#pick-element-btn')).toContainText('Cancel capture');
  await expectLocator(sidePanel.locator('#pick-element-btn')).toHaveClass(/cancel/);

  await sidePanel.close();
});

test('denied permission grant leaves the alert visible with a denial message (firefox)', async ({
  newExtensionPage,
}) => {
  const sidePanel = await openSidePanelWithStubs(newExtensionPage);

  await sidePanel.evaluate(() => {
    const w = window as unknown as Record<string, unknown>;
    w.__haviResponder = (msg: { type?: string }) => {
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
    w.__haviPermissionsResponder = () => false;
  });

  await sidePanel.locator('#capture-btn').click();
  await sidePanel.locator('#capture-alert-action').click();

  await expectLocator(sidePanel.locator('#capture-alert')).toBeVisible();
  await expectLocator(sidePanel.locator('#capture-alert-message')).toContainText('not granted');

  await sidePanel.close();
});

test('classifyCaptureFailure returns the shape the side panel relies on (firefox)', async ({
  backgroundPage,
}) => {
  const cases = await backgroundPage.evaluate(() => {
    const g = globalThis as unknown as {
      classifyCaptureFailure?: (e: unknown, t: unknown) => unknown;
    };
    if (!g.classifyCaptureFailure) return { error: 'classifyCaptureFailure not on globalThis' };

    const permError = new Error("Either the '<all_urls>' or 'activeTab' permission is required.");
    return {
      permWithUrl: g.classifyCaptureFailure(permError, { url: 'https://example.com/page' }),
      permWithoutUrl: g.classifyCaptureFailure(permError, { url: undefined }),
      chromeUrl: g.classifyCaptureFailure(new Error('Cannot capture this page'), { url: 'chrome://newtab/' }),
      otherWithUrl: g.classifyCaptureFailure(new Error('Server unreachable'), { url: 'https://example.com/' }),
    };
  });

  expect(cases).toMatchObject({
    permWithUrl: { ok: false, code: 'permission_required', origin: 'https://example.com' },
    permWithoutUrl: { ok: false, code: 'permission_required', origin: null },
    chromeUrl: { ok: false, code: 'unsupported_page', origin: null },
    otherWithUrl: { ok: false, code: 'other', origin: 'https://example.com' },
  });
});
