import { test, expect, expectLocator } from './firefox-fixtures';

const SERVER_URL = process.env.HAVI_SERVER_URL ?? 'http://localhost:8090';

const TINY_PNG_DATA_URL =
  'data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR4nGNgAAIAAAUAAeImBZsAAAAASUVORK5CYII=';

test('Go server is reachable (firefox)', async ({ request }) => {
  const resp = await request.get(`${SERVER_URL}/health`);
  expect(resp.status()).toBe(200);
});

test('extension loads and content script injects on a localhost page (firefox)', async ({
  context,
  marionette: _marionette,
  testPageUrl,
}) => {
  // Verify content_script injection by observing the side effect content.js produces
  // on its target document. The Chrome variant of this test also pings the content
  // script from the side panel, but the Firefox WebExtension API cannot see Playwright
  // tab URLs even with `<all_urls>` granted (the manifest declares `host_permissions`
  // restricted to port-80 localhost/127.0.0.1 and the extension lacks the `tabs`
  // permission). The injection-side proof — content.js running and writing
  // `annBufferSize` via `chrome.storage.sync.get` — is what this test actually cares
  // about; see firefox-permission-flow.spec.ts for end-to-end message-flow coverage.
  const page = await context.newPage();
  await page.goto(testPageUrl + '/');
  await expect(page.locator('#target')).toBeVisible();

  await expect
    .poll(async () => page.evaluate(() => document.documentElement.dataset.annBufferSize ?? null), {
      message: 'content_script never wrote annBufferSize on the test page',
      timeout: 10_000,
    })
    .not.toBeNull();

  await page.close();
});

test('side panel page drives create-annotation through the background into the Go server (firefox)', async ({
  gotoExtension,
  testPageUrl,
  request,
}) => {
  const sidePanel = await gotoExtension('/src/sidepanel/sidepanel.html');
  await sidePanel.waitForFunction(() => typeof chrome !== 'undefined' && !!chrome.runtime?.id);

  const annotation = {
    '@context': 'http://www.w3.org/ns/anno.jsonld',
    type: 'Annotation',
    motivation: 'commenting',
    creator: { type: 'Person', name: 'playwright-firefox-e2e' },
    body: [{ type: 'TextualBody', value: 'Playwright firefox extension-flow test', purpose: 'commenting' }],
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
    async (payload: unknown) => {
      const { annotation, imageDataUrl } = payload as { annotation: unknown; imageDataUrl: string };
      return await new Promise((resolve) => {
        chrome.runtime.sendMessage(
          { type: 'create-annotation', data: { annotation, imageDataUrl } },
          (response: unknown) => resolve(response),
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

// expectLocator is re-exported for permission-flow tests; reference it here so unused-import lint passes.
void expectLocator;
