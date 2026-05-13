import type { MarionetteClient } from './marionette';

type ScriptArg = unknown;

function unwrap(res: unknown): unknown {
  if (res && typeof res === 'object' && 'value' in (res as Record<string, unknown>)) {
    return (res as { value: unknown }).value;
  }
  return res;
}

/**
 * Wraps a Marionette content window so tests can drive a moz-extension:// page that
 * Playwright's juggler protocol cannot navigate to. Each script call switches to
 * the owned handle, then unwraps Xrays so `window`, `document`, `chrome`, and
 * `browser` inside test code resolve against the real page principal — matching
 * Playwright Page semantics.
 */
export class MarionettePage {
  private initScripts: string[] = [];

  constructor(public readonly mar: MarionetteClient, public readonly handle: string) {}

  private async switchTo(): Promise<void> {
    await this.mar.send('Marionette:SetContext', { value: 'content' });
    await this.mar.send('WebDriver:SwitchToWindow', { handle: this.handle });
  }

  /**
   * Wrap a user-supplied function (or statement-body string) so it runs against
   * the real (Xray-unwrapped) page window. We inline aliases for `window`,
   * `document`, `chrome`, and `browser` so test bodies look identical to their
   * Playwright counterparts.
   *
   * Function source: invoked with the user-passed args.
   * String source: treated as a statement body that `return`s a value.
   */
  private wrapBody(fnOrSrc: string | ((...args: unknown[]) => unknown), awaitable: boolean): string {
    const isFn = typeof fnOrSrc !== 'string';
    const fnSrc = isFn ? (fnOrSrc as () => unknown).toString() : (fnOrSrc as string);

    const argsLine = awaitable
      ? `var __cb = arguments[arguments.length - 1];
         var __args = Array.prototype.slice.call(arguments, 0, arguments.length - 1);`
      : `var __args = Array.prototype.slice.call(arguments);`;

    const inner = isFn
      ? `(${fnSrc}).apply(null, __args)`
      : `(function(){ ${fnSrc} }).call(null)`;

    const invoke = awaitable
      ? `try {
           var __r = ${inner};
           if (__r && typeof __r.then === 'function') {
             __r.then(function(v){ __cb(v); }, function(e){ __cb({ __haviErr: String(e && e.stack || e) }); });
           } else {
             __cb(__r);
           }
         } catch (e) {
           __cb({ __haviErr: String(e && e.stack || e) });
         }`
      : `return ${inner};`;

    // In sandbox scripts run via Marionette, plain expando assignments on `window`
    // do not survive across calls (each ExecuteScript gets a fresh Xray waiver).
    // We route assignments to `window.__havi*` properties through a persistent
    // expando bucket on `document` and read them back from the same bucket, so
    // test code can pretend `window.__havi*` is a shared store like in Chromium.
    const aliasProlog = `
      if (!document.__haviStash) document.__haviStash = {};
      var __stash = document.__haviStash;
      var __windowProxy = new Proxy(window, {
        get: function(t, k) {
          if (typeof k === 'string' && k.indexOf('__havi') === 0 && k in __stash) return __stash[k];
          return t[k];
        },
        set: function(t, k, v) {
          if (typeof k === 'string' && k.indexOf('__havi') === 0) { __stash[k] = v; return true; }
          try { t[k] = v; } catch(_) {}
          return true;
        },
        has: function(t, k) {
          if (typeof k === 'string' && k.indexOf('__havi') === 0) return k in __stash;
          return k in t;
        },
      });
    `;
    return `
      ${argsLine}
      var __name = function(fn) { return fn; };
      var __publicField = function(obj, k, v) { obj[k] = v; return v; };
      var window = XPCNativeWrapper.unwrap(this);
      var document = window.document;
      var chrome = window.chrome;
      var browser = window.browser;
      ${aliasProlog}
      window = __windowProxy;
      ${invoke}
    `;
  }

  private async runScript(script: string, args: ScriptArg[] = [], timeoutMs = 30_000): Promise<unknown> {
    await this.switchTo();
    const res = await this.mar.send(
      'WebDriver:ExecuteScript',
      { script, args },
      timeoutMs,
    );
    return unwrap(res);
  }

  private async runAsyncScript(script: string, args: ScriptArg[] = [], timeoutMs = 30_000): Promise<unknown> {
    await this.switchTo();
    await this.mar.send('WebDriver:SetTimeouts', { script: timeoutMs });
    const res = await this.mar.send(
      'WebDriver:ExecuteAsyncScript',
      { script, args },
      timeoutMs + 5_000,
    );
    const v = unwrap(res);
    if (v && typeof v === 'object' && '__haviErr' in (v as Record<string, unknown>)) {
      throw new Error('evaluate error: ' + (v as { __haviErr: string }).__haviErr);
    }
    return v;
  }

  url(): Promise<string> {
    return this.runScript(
      this.wrapBody('return document.URL;', false),
    ) as Promise<string>;
  }

  async goto(url: string, opts: { timeout?: number } = {}): Promise<void> {
    await this.switchTo();
    const timeout = opts.timeout ?? 30_000;
    await this.mar.send('WebDriver:SetTimeouts', { pageLoad: timeout });
    await this.mar.send('WebDriver:Navigate', { url }, timeout + 5_000);
    for (const s of this.initScripts) {
      await this.runScript(s);
    }
  }

  async addInitScript(fnOrSrc: string | ((...args: unknown[]) => void)): Promise<void> {
    const body = this.wrapBody(fnOrSrc, false);
    this.initScripts.push(body);
  }

  /**
   * Runs a function as if in the page context. The function may be sync or async.
   * `window`, `document`, `chrome`, `browser` resolve to the real (unwrapped) page globals.
   */
  async evaluate<T = unknown>(fn: ((arg?: unknown) => T | Promise<T>) | string, arg?: unknown): Promise<T> {
    const body = this.wrapBody(fn, true);
    const result = await this.runAsyncScript(body, [arg]);
    return result as T;
  }

  async waitForFunction(
    fn: (() => unknown) | string,
    opts: { timeout?: number; pollMs?: number } = {},
  ): Promise<void> {
    const timeoutMs = opts.timeout ?? 10_000;
    const pollMs = opts.pollMs ?? 100;
    const deadline = Date.now() + timeoutMs;
    const body = this.wrapBody(fn, false);
    const wrapper = `try { ${body} } catch (e) { return false; }`;
    while (Date.now() < deadline) {
      const ok = await this.runScript(wrapper);
      if (ok) return;
      await new Promise((r) => setTimeout(r, pollMs));
    }
    const repr = typeof fn === 'string' ? fn : fn.toString();
    throw new Error('waitForFunction timed out: ' + repr.slice(0, 120));
  }

  locator(selector: string): MarionetteLocator {
    return new MarionetteLocator(this, selector);
  }

  async close(): Promise<void> {
    try {
      await this.switchTo();
      await this.mar.send('WebDriver:CloseWindow', {});
    } catch {
      /* tab may already be gone */
    }
  }
}

export class MarionetteLocator {
  constructor(public readonly page: MarionettePage, public readonly selector: string) {}

  async click(opts: { timeout?: number } = {}): Promise<void> {
    const timeoutMs = opts.timeout ?? 10_000;
    await this.page.waitForFunction(
      `return !!document.querySelector(${JSON.stringify(this.selector)});`,
      { timeout: timeoutMs },
    );
    await this.page.evaluate(
      (sel: unknown) => {
        const el = document.querySelector(sel as string) as HTMLElement | null;
        if (!el) throw new Error('element not found: ' + sel);
        el.click();
      },
      this.selector,
    );
  }

  async count(): Promise<number> {
    const n = await this.page.evaluate(
      (sel: unknown) => document.querySelectorAll(sel as string).length,
      this.selector,
    );
    return n as number;
  }

  async innerText(): Promise<string> {
    const t = await this.page.evaluate(
      (sel: unknown) => {
        const el = document.querySelector(sel as string) as HTMLElement | null;
        return el ? el.innerText : '';
      },
      this.selector,
    );
    return t as string;
  }

  async isVisible(): Promise<boolean> {
    const v = await this.page.evaluate(
      (sel: unknown) => {
        const el = document.querySelector(sel as string) as HTMLElement | null;
        if (!el) return false;
        const style = getComputedStyle(el);
        if (style.display === 'none' || style.visibility === 'hidden' || style.opacity === '0') return false;
        const rect = el.getBoundingClientRect();
        return rect.width > 0 && rect.height > 0;
      },
      this.selector,
    );
    return v as boolean;
  }

  async getClass(): Promise<string> {
    const c = await this.page.evaluate(
      (sel: unknown) => {
        const el = document.querySelector(sel as string) as HTMLElement | null;
        return el ? el.className : '';
      },
      this.selector,
    );
    return c as string;
  }

  first(): MarionetteLocator {
    return this;
  }
}

export class MarionetteExpect {
  constructor(private readonly locator: MarionetteLocator, private readonly negated: boolean = false) {}

  get not(): MarionetteExpect {
    return new MarionetteExpect(this.locator, !this.negated);
  }

  private async poll(check: () => Promise<boolean>, what: string, timeoutMs = 10_000): Promise<void> {
    const deadline = Date.now() + timeoutMs;
    let last = false;
    while (Date.now() < deadline) {
      try {
        last = await check();
        if (this.negated ? !last : last) return;
      } catch {
        /* keep polling */
      }
      await new Promise((r) => setTimeout(r, 100));
    }
    throw new Error(
      `expect(${this.locator.selector})${this.negated ? '.not' : ''}.${what} failed (last=${last})`,
    );
  }

  async toBeVisible(opts: { timeout?: number } = {}): Promise<void> {
    await this.poll(() => this.locator.isVisible(), 'toBeVisible', opts.timeout);
  }

  async toBeHidden(opts: { timeout?: number } = {}): Promise<void> {
    await this.poll(async () => !(await this.locator.isVisible()), 'toBeHidden', opts.timeout);
  }

  async toContainText(text: string, opts: { timeout?: number } = {}): Promise<void> {
    await this.poll(
      async () => {
        const t = await this.locator.innerText().catch(() => '');
        return t.includes(text);
      },
      `toContainText(${JSON.stringify(text)})`,
      opts.timeout,
    );
  }

  async toHaveText(text: string, opts: { timeout?: number } = {}): Promise<void> {
    await this.poll(
      async () => {
        const t = await this.locator.innerText().catch(() => '');
        return t.trim() === text;
      },
      `toHaveText(${JSON.stringify(text)})`,
      opts.timeout,
    );
  }

  async toHaveClass(re: RegExp, opts: { timeout?: number } = {}): Promise<void> {
    await this.poll(
      async () => {
        const c = await this.locator.getClass().catch(() => '');
        return re.test(c);
      },
      `toHaveClass(${re})`,
      opts.timeout,
    );
  }
}

export function expectLocator(locator: MarionetteLocator): MarionetteExpect {
  return new MarionetteExpect(locator);
}
