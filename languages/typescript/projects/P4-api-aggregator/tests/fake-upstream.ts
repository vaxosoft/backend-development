import { createServer, type Server } from 'node:http';
import type { AddressInfo } from 'node:net';

export interface Behavior {
  status?: number;
  delayMs?: number;
  /** Destroy the connection without answering. */
  drop?: boolean;
  /** Never answer; the request ends only when the client cancels it. */
  stall?: boolean;
  /** Raw body to send instead of the default JSON item. */
  body?: string;
}

export type Script = (id: number, attempt: number) => Behavior;

/** A real loopback HTTP server that plays the upstream and records what the app does to it. */
export class FakeUpstream {
  script: Script = () => ({});
  inFlight = 0;
  maxInFlight = 0;
  private readonly attempts = new Map<number, number>();
  private readonly cancelledIds: number[] = [];
  private readonly server: Server;

  private constructor() {
    this.server = createServer((req, res) => {
      const id = Number(/^\/items\/(\d+)$/.exec(req.url ?? '')?.[1]);
      if (Number.isNaN(id)) {
        res.writeHead(404).end();
        return;
      }
      const attempt = (this.attempts.get(id) ?? 0) + 1;
      this.attempts.set(id, attempt);
      this.inFlight++;
      this.maxInFlight = Math.max(this.maxInFlight, this.inFlight);

      const timer: { handle?: NodeJS.Timeout } = {};
      res.once('close', () => {
        this.inFlight--;
        clearTimeout(timer.handle);
        if (!res.writableFinished) {
          this.cancelledIds.push(id);
        }
      });

      const behavior = this.script(id, attempt);
      if (behavior.drop) {
        req.socket.destroy();
        return;
      }
      if (behavior.stall) {
        return;
      }
      timer.handle = setTimeout(() => {
        const body = behavior.body ?? JSON.stringify({ id, name: `item-${id}` });
        res.writeHead(behavior.status ?? 200, { 'content-type': 'application/json' }).end(body);
      }, behavior.delayMs ?? 0);
    });
  }

  static async start(): Promise<FakeUpstream> {
    const upstream = new FakeUpstream();
    await new Promise<void>((resolve) => upstream.server.listen(0, '127.0.0.1', resolve));
    return upstream;
  }

  get baseUrl(): string {
    const { port } = this.server.address() as AddressInfo;
    return `http://127.0.0.1:${port}`;
  }

  /** How many requests for this id reached the upstream (all attempts). */
  calls(id: number): number {
    return this.attempts.get(id) ?? 0;
  }

  get totalCalls(): number {
    return [...this.attempts.values()].reduce((sum, n) => sum + n, 0);
  }

  /** Ids whose request was cut off by the client before the upstream finished answering. */
  get cancelled(): number[] {
    return [...this.cancelledIds];
  }

  async close(): Promise<void> {
    this.server.closeAllConnections();
    await new Promise<void>((resolve) => this.server.close(() => resolve()));
  }
}
