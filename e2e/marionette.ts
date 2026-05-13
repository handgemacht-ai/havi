import { Socket, connect } from 'node:net';

type Response = { error: unknown; result: unknown };

export class MarionetteClient {
  private socket: Socket | null = null;
  private buffer: Buffer = Buffer.alloc(0);
  private pending: Map<number, (resp: Response) => void> = new Map();
  private nextId = 1;
  private helloSeen = false;
  private helloWaiters: Array<() => void> = [];

  async connect(host: string, port: number, timeoutMs = 20_000): Promise<void> {
    const deadline = Date.now() + timeoutMs;
    let lastErr: Error | null = null;
    while (Date.now() < deadline) {
      try {
        await this.tryConnect(host, port);
        await this.waitForHello();
        return;
      } catch (e) {
        lastErr = e as Error;
        await new Promise((r) => setTimeout(r, 250));
      }
    }
    throw new Error(`Marionette connect failed: ${lastErr?.message ?? 'timeout'}`);
  }

  private tryConnect(host: string, port: number): Promise<void> {
    return new Promise((resolve, reject) => {
      const sock = connect({ host, port });
      const onErr = (e: Error) => {
        sock.destroy();
        reject(e);
      };
      sock.once('error', onErr);
      sock.once('connect', () => {
        sock.removeListener('error', onErr);
        this.socket = sock;
        sock.on('data', (c) => this.onData(c));
        sock.on('error', (e) => this.onSocketError(e));
        resolve();
      });
    });
  }

  private waitForHello(): Promise<void> {
    if (this.helloSeen) return Promise.resolve();
    return new Promise<void>((resolve, reject) => {
      const timer = setTimeout(() => reject(new Error('marionette hello timeout')), 5_000);
      this.helloWaiters.push(() => {
        clearTimeout(timer);
        resolve();
      });
    });
  }

  private onSocketError(e: Error): void {
    for (const cb of this.pending.values()) cb({ error: e.message, result: null });
    this.pending.clear();
  }

  private onData(chunk: Buffer): void {
    this.buffer = Buffer.concat([this.buffer, chunk]);
    while (true) {
      let colonIdx = -1;
      for (let i = 0; i < this.buffer.length && i < 12; i++) {
        if (this.buffer[i] === 0x3a) {
          colonIdx = i;
          break;
        }
        if (this.buffer[i] < 0x30 || this.buffer[i] > 0x39) {
          throw new Error(`marionette frame: unexpected byte 0x${this.buffer[i].toString(16)}`);
        }
      }
      if (colonIdx === -1) return;
      const len = parseInt(this.buffer.subarray(0, colonIdx).toString('ascii'), 10);
      const start = colonIdx + 1;
      if (this.buffer.length < start + len) return;
      const json = this.buffer.subarray(start, start + len).toString('utf8');
      this.buffer = this.buffer.subarray(start + len);
      let msg: unknown;
      try {
        msg = JSON.parse(json);
      } catch (e) {
        throw new Error(`marionette frame: invalid JSON: ${(e as Error).message}: ${json}`);
      }
      this.dispatch(msg);
    }
  }

  private dispatch(msg: unknown): void {
    if (!this.helloSeen && msg && typeof msg === 'object' && (msg as { applicationType?: string }).applicationType) {
      this.helloSeen = true;
      for (const w of this.helloWaiters) w();
      this.helloWaiters = [];
      return;
    }
    if (Array.isArray(msg) && msg[0] === 1) {
      const [, msgId, error, result] = msg as [number, number, unknown, unknown];
      const cb = this.pending.get(msgId);
      if (cb) {
        this.pending.delete(msgId);
        cb({ error, result });
      }
    }
  }

  async send(name: string, params: Record<string, unknown>, timeoutMs = 30_000): Promise<unknown> {
    if (!this.socket) throw new Error('marionette not connected');
    if (!this.helloSeen) await this.waitForHello();
    const id = this.nextId++;
    const frame = JSON.stringify([0, id, name, params]);
    const out = Buffer.from(`${Buffer.byteLength(frame, 'utf8')}:${frame}`, 'utf8');
    return new Promise((resolve, reject) => {
      const timer = setTimeout(() => {
        this.pending.delete(id);
        reject(new Error(`marionette ${name} timed out after ${timeoutMs}ms`));
      }, timeoutMs);
      this.pending.set(id, ({ error, result }) => {
        clearTimeout(timer);
        if (error) reject(new Error(`marionette ${name} failed: ${JSON.stringify(error)}`));
        else resolve(result);
      });
      this.socket!.write(out);
    });
  }

  async newSession(): Promise<unknown> {
    return this.send('WebDriver:NewSession', { capabilities: { alwaysMatch: {} } });
  }

  async installAddon(path: string, temporary = true): Promise<string> {
    const result = (await this.send('Addon:Install', { path, temporary })) as
      | { value?: string }
      | string;
    if (typeof result === 'string') return result;
    return result?.value ?? '';
  }

  async setChromeContext(): Promise<void> {
    await this.send('Marionette:SetContext', { value: 'chrome' });
  }

  async executeScriptInChrome(script: string): Promise<unknown> {
    const res = (await this.send('WebDriver:ExecuteScript', {
      script,
      args: [],
      newSandbox: true,
    })) as { value?: unknown } | unknown;
    if (res && typeof res === 'object' && 'value' in (res as Record<string, unknown>)) {
      return (res as { value: unknown }).value;
    }
    return res;
  }

  close(): void {
    this.socket?.destroy();
    this.socket = null;
  }
}
