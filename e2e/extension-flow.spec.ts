import { test, expect } from './fixtures';

const SERVER_URL = process.env.HAVI_SERVER_URL ?? 'http://localhost:8090';

const TINY_PNG_DATA_URL =
  'data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR4nGNgAAIAAAUAAeImBZsAAAAASUVORK5CYII=';

test('Go server is reachable', async ({ request }) => {
  const resp = await request.get(`${SERVER_URL}/health`);
  expect(resp.status()).toBe(200);
});

test('extension SW registers and content script injects on a localhost page', async ({
  context,
  serviceWorker,
  testPageUrl,
}) => {
  expect(serviceWorker.url()).toMatch(/background\/background\.js$/);
  const extensionId = new URL(serviceWorker.url()).host;

  const page = await context.newPage();
  await page.goto(testPageUrl + '/');
  await expect(page.locator('#target')).toBeVisible();

  const sidePanel = await context.newPage();
  await sidePanel.goto(`chrome-extension://${extensionId}/src/sidepanel/sidepanel.html`);
  await sidePanel.waitForFunction(() => typeof chrome !== 'undefined' && !!chrome.runtime?.id);

  const probe = await sidePanel.evaluate(async (urlPrefix) => {
    for (let i = 0; i < 25; i++) {
      const tabs = await chrome.tabs.query({ url: urlPrefix + '/*' });
      const tab = tabs[0];
      if (tab?.id) {
        try {
          const resp = await chrome.tabs.sendMessage(tab.id, { type: 'ping' });
          return { ok: true, resp };
        } catch {
          /* not ready yet */
        }
      }
      await new Promise((r) => setTimeout(r, 200));
    }
    return { ok: false, error: 'timeout' };
  }, testPageUrl);

  expect(probe, JSON.stringify(probe)).toMatchObject({ ok: true, resp: { ok: true } });

  await sidePanel.close();
  await page.close();
});

test('side panel page drives create-annotation through the SW into the Go server', async ({
  context,
  serviceWorker,
  testPageUrl,
  request,
}) => {
  const extensionId = new URL(serviceWorker.url()).host;

  const sidePanel = await context.newPage();
  await sidePanel.goto(`chrome-extension://${extensionId}/src/sidepanel/sidepanel.html`);
  await sidePanel.waitForFunction(() => typeof chrome !== 'undefined' && !!chrome.runtime?.id);

  const annotation = {
    '@context': 'http://www.w3.org/ns/anno.jsonld',
    type: 'Annotation',
    motivation: 'commenting',
    creator: { type: 'Person', name: 'playwright-e2e' },
    body: [{ type: 'TextualBody', value: 'Playwright extension-flow test', purpose: 'commenting' }],
    target: {
      source: testPageUrl + '/',
      selector: [
        {
          type: 'FragmentSelector',
          conformsTo: 'http://www.w3.org/TR/media-frags/',
          value: 'xywh=10,20,320,120',
        },
        { type: 'CssSelector', value: '#target' },
      ],
      state: { type: 'HttpRequestState', value: 'viewport=1280x720' },
    },
  };

  const result = await sidePanel.evaluate(
    async ({ annotation, imageDataUrl }) => {
      return await new Promise((resolve) => {
        chrome.runtime.sendMessage(
          { type: 'create-annotation', data: { annotation, imageDataUrl } },
          (response) => resolve(response),
        );
      });
    },
    { annotation, imageDataUrl: TINY_PNG_DATA_URL },
  );

  expect(result).toMatchObject({ ok: true });
  const data = (result as { data: { id: string; annotation: { target: { source: string } } } }).data;
  expect(data.id).toMatch(/^[0-9a-f-]{36}$/);
  expect(data.annotation.target.source).toBe(testPageUrl + '/');

  const get = await request.get(`${SERVER_URL}/api/annotations/${data.id}`);
  expect(get.status()).toBe(200);
  const body = await get.json();
  expect(body.data.id).toBe(data.id);

  const img = await request.get(`${SERVER_URL}/api/annotations/${data.id}/image`);
  expect(img.status()).toBe(200);
  expect(img.headers()['content-type']).toContain('image/png');

  const del = await request.delete(`${SERVER_URL}/api/annotations/${data.id}`);
  expect(del.status()).toBe(204);

  await sidePanel.close();
});
