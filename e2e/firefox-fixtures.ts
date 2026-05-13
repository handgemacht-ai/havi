import { test as base, firefox, type BrowserContext, type Page } from '@playwright/test';
import { createServer, type Server } from 'node:http';
import { readFile, mkdtemp, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { execFileSync } from 'node:child_process';
import { existsSync } from 'node:fs';
import { join, resolve } from 'node:path';
import { AddressInfo } from 'node:net';
import { MarionetteClient } from './marionette';
import { MarionettePage, expectLocator } from './marionette-page';

const REPO_ROOT = resolve(__dirname, '..');
const EXTENSION_BUILD = resolve(REPO_ROOT, 'firefox-build');
const ADDON_ID = 'havi@handgemacht.ai';
const ADDON_UUID = '00000000-0000-0000-0000-0000000000a1';
const MARIONETTE_PORT = Number(process.env.HAVI_FF_MARIONETTE_PORT ?? 2828);

type Fixtures = {
  context: BrowserContext;
  marionette: MarionetteClient;
  extensionId: string;
  /** Open a moz-extension page in a new tab (or reuse the placeholder) and return a Marionette-driven page. */
  gotoExtension: (path: string) => Promise<MarionettePage>;
  /**
   * Open a fresh tab attached to Marionette without navigating yet. Useful when
   * tests need to install init scripts before the page loads.
   */
  newExtensionPage: () => Promise<MarionettePage>;
  /** Background-page handle for the extension (Firefox MV2-style auto-generated background). */
  backgroundPage: MarionettePage;
  testPageUrl: string;
};

function ensureBuild() {
  if (!existsSync(join(EXTENSION_BUILD, 'manifest.json'))) {
    execFileSync('bash', [resolve(REPO_ROOT, 'scripts/build-firefox.sh')], {
      stdio: 'inherit',
    });
  }
}

function unwrapMar(res: unknown): unknown {
  if (res && typeof res === 'object' && 'value' in (res as Record<string, unknown>)) {
    return (res as { value: unknown }).value;
  }
  return res;
}

export const test = base.extend<Fixtures>({
  context: async ({}, use) => {
    ensureBuild();
    const userDataDir = await mkdtemp(join(tmpdir(), 'havi-ff-pw-'));
    const context = await firefox.launchPersistentContext(userDataDir, {
      headless: true,
      args: ['-marionette', '-remote-allow-system-access'],
      firefoxUserPrefs: {
        'marionette.port': MARIONETTE_PORT,
        'marionette.enabled': true,
        'extensions.webextensions.uuids': JSON.stringify({ [ADDON_ID]: ADDON_UUID }),
        'extensions.autoDisableScopes': 0,
        'extensions.enabledScopes': 15,
        'browser.shell.checkDefaultBrowser': false,
        'browser.startup.homepage_override.mstone': 'ignore',
        'datareporting.healthreport.uploadEnabled': false,
        'datareporting.policy.dataSubmissionEnabled': false,
      },
    });
    await use(context);
    await context.close();
    await rm(userDataDir, { recursive: true, force: true }).catch(() => {});
  },

  marionette: async ({ context: _context }, use) => {
    const mar = new MarionetteClient();
    await mar.connect('127.0.0.1', MARIONETTE_PORT);
    await mar.newSession();
    await mar.installAddon(EXTENSION_BUILD, true);
    await waitForExtensionPolicy(mar);
    await use(mar);
    mar.close();
  },

  extensionId: async ({ marionette: _m }, use) => {
    await use(ADDON_UUID);
  },

  newExtensionPage: async ({ marionette, context }, use) => {
    const opened: MarionettePage[] = [];
    const helper = async (): Promise<MarionettePage> => {
      await marionette.send('Marionette:SetContext', { value: 'content' });
      const handlesBefore = unwrapMar(await marionette.send('WebDriver:GetWindowHandles', {})) as string[];
      let handle: string;
      if (opened.length === 0 && handlesBefore.length > 0) {
        handle = handlesBefore[handlesBefore.length - 1];
        await marionette.send('WebDriver:SwitchToWindow', { handle });
      } else {
        const newWinRes = unwrapMar(
          await marionette.send('WebDriver:NewWindow', { type: 'tab', focus: true }),
        ) as { handle: string };
        handle = newWinRes.handle;
        await marionette.send('WebDriver:SwitchToWindow', { handle });
      }
      if (context.pages().length === 0) {
        await context.newPage();
      }
      const page = new MarionettePage(marionette, handle);
      opened.push(page);
      return page;
    };
    await use(helper);
    for (const p of opened) await p.close().catch(() => {});
  },

  gotoExtension: async ({ newExtensionPage }, use) => {
    const helper = async (path: string): Promise<MarionettePage> => {
      const page = await newExtensionPage();
      const url = `moz-extension://${ADDON_UUID}${path.startsWith('/') ? path : '/' + path}`;
      await page.goto(url);
      return page;
    };
    await use(helper);
  },

  backgroundPage: async ({ gotoExtension }, use) => {
    // Firefox WebExtensions with manifest_version 2 expose the background scripts page at
    // `moz-extension://<uuid>/_generated_background_page.html`. We can navigate to it
    // like any other extension page (it stays alive while the extension is loaded).
    const page = await gotoExtension('/_generated_background_page.html');
    await use(page);
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

async function waitForExtensionPolicy(mar: MarionetteClient): Promise<void> {
  await mar.send('Marionette:SetContext', { value: 'chrome' });
  const deadline = Date.now() + 15_000;
  while (Date.now() < deadline) {
    const r = unwrapMar(
      await mar.send('WebDriver:ExecuteScript', {
        script: `
          const p = WebExtensionPolicy.getByID(${JSON.stringify(ADDON_ID)});
          return p ? { active: p.active, baseURL: p.getURL('') } : null;
        `,
        args: [],
      }),
    ) as { active: boolean; baseURL: string } | null;
    if (r && r.active) {
      await grantAllUrls(mar);
      await mar.send('Marionette:SetContext', { value: 'content' });
      return;
    }
    await new Promise((r) => setTimeout(r, 200));
  }
  throw new Error('Extension policy never became active');
}

/**
 * Firefox temporary-installed extensions don't auto-grant optional permissions like
 * Chrome unpacked installs do, so content_scripts that target `<all_urls>` only run
 * on origins covered by host_permissions. We approximate Chrome's behavior by adding
 * the optional `<all_urls>` permission via the internal ExtensionPermissions API, then
 * reloading the extension so the policy picks up the granted origins.
 */
async function grantAllUrls(mar: MarionetteClient): Promise<void> {
  const result = unwrapMar(
    await mar.send('WebDriver:ExecuteAsyncScript', {
      script: `
        const cb = arguments[arguments.length - 1];
        (async () => {
          try {
            const { ExtensionPermissions } = ChromeUtils.importESModule(
              'resource://gre/modules/ExtensionPermissions.sys.mjs'
            );
            await ExtensionPermissions.add(
              ${JSON.stringify(ADDON_ID)},
              { permissions: [], origins: ['<all_urls>'] }
            );
            const policy = WebExtensionPolicy.getByID(${JSON.stringify(ADDON_ID)});
            const extension = policy && policy.extension;
            if (extension && typeof extension.updatePermissions === 'function') {
              await extension.updatePermissions({ permissions: [], origins: ['<all_urls>'] }, 'add');
            }
            if (extension && typeof extension.updateContentScripts === 'function') {
              try { await extension.updateContentScripts(); } catch (_) {}
            }
            // Allow the extension to run in private windows — Playwright's persistent
            // context sometimes opens pages with a private principal.
            if (extension) {
              try { extension.privateBrowsingAllowed = true; } catch (_) {}
              try {
                const { ExtensionParent } = ChromeUtils.importESModule(
                  'resource://gre/modules/ExtensionParent.sys.mjs'
                );
                if (ExtensionParent && ExtensionParent.PrivateBrowsingMixin) {
                  ExtensionParent.PrivateBrowsingMixin.setAllowedToRun(extension, true);
                }
              } catch (_) {}
            }
            cb({ ok: true });
          } catch (e) {
            cb({ ok: false, err: String(e) });
          }
        })();
      `,
      args: [],
    }),
  ) as { ok: boolean; err?: string; allowedOrigins?: string[] };
  if (!result.ok) {
    throw new Error('grantAllUrls failed: ' + result.err);
  }
}

export const expect = test.expect;
export { expectLocator };
export type { MarionettePage };
