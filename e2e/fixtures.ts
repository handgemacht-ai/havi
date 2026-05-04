import { test as base, chromium, type BrowserContext, type Worker } from '@playwright/test';
import { createServer, type Server } from 'node:http';
import { readFile } from 'node:fs/promises';
import { mkdtemp, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';
import { AddressInfo } from 'node:net';

const EXTENSION_PATH = resolve(__dirname, '..', 'extension');

type Fixtures = {
  context: BrowserContext;
  serviceWorker: Worker;
  testPageUrl: string;
};

export const test = base.extend<Fixtures>({
  context: async ({}, use) => {
    const userDataDir = await mkdtemp(join(tmpdir(), 'havi-pw-'));
    const context = await chromium.launchPersistentContext(userDataDir, {
      headless: true,
      channel: 'chromium',
      args: [
        `--disable-extensions-except=${EXTENSION_PATH}`,
        `--load-extension=${EXTENSION_PATH}`,
        '--no-first-run',
        '--no-default-browser-check',
      ],
    });
    await use(context);
    await context.close();
    await rm(userDataDir, { recursive: true, force: true });
  },

  serviceWorker: async ({ context }, use) => {
    let [sw] = context.serviceWorkers();
    if (!sw) sw = await context.waitForEvent('serviceworker', { timeout: 10_000 });
    await use(sw);
  },

  testPageUrl: async ({}, use) => {
    const server: Server = createServer(async (req, res) => {
      const path = req.url === '/' ? '/index.html' : req.url || '/index.html';
      try {
        const body = await readFile(resolve(__dirname, 'test-page', '.' + path));
        res.writeHead(200, { 'content-type': path.endsWith('.html') ? 'text/html' : 'text/plain' });
        res.end(body);
      } catch {
        res.writeHead(404);
        res.end('not found');
      }
    });
    server.keepAliveTimeout = 1;
    await new Promise<void>((r) => server.listen(0, '127.0.0.1', r));
    const port = (server.address() as AddressInfo).port;
    await use(`http://127.0.0.1:${port}`);
    server.closeAllConnections();
    await new Promise<void>((r) => server.close(() => r()));
  },
});

export const expect = test.expect;
