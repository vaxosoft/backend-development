import { mkdtempSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { setTimeout as sleep } from 'node:timers/promises';
import type { FastifyInstance, LightMyRequestResponse } from 'fastify';
import { buildApp } from '../starter/src/app.ts';
import type { Config } from '../starter/src/config.ts';

export const SECRET = 'test-secret-that-is-at-least-32-characters-long';

export interface TestApp {
  app: FastifyInstance;
  /** Parsed access-log lines (msg "request") written so far. */
  logs: () => Record<string, unknown>[];
  /** Raw text of everything logged. */
  rawLogs: () => string;
  /** Moves the clock used by the rate limiter. */
  advance: (ms: number) => void;
  dbPath: string;
  close: () => Promise<void>;
}

/** Builds an isolated app: own SQLite database, fake clock, log collector, cheap password hashing. */
export async function startApp(overrides: Partial<Config> = {}): Promise<TestApp> {
  const dir = mkdtempSync(join(tmpdir(), 'p5-'));
  const dbPath = join(dir, 'shortener.db');
  const lines: string[] = [];
  let now = 1_000_000;
  const app = await buildApp({
    jwtSecret: SECRET,
    dbPath,
    rateLimitBurst: 5,
    rateLimitRefillPerSecond: 1,
    scryptCost: 1024,
    now: () => now,
    logStream: { write: (line) => lines.push(line) },
    ...overrides,
  });
  return {
    app,
    logs: () => lines.map((line) => JSON.parse(line)).filter((entry) => entry.msg === 'request'),
    rawLogs: () => lines.join(''),
    advance: (ms) => {
      now += ms;
    },
    dbPath,
    close: async () => {
      await app.close();
      rmSync(dir, { recursive: true, force: true });
    },
  };
}

export function bearer(token: string) {
  return { authorization: `Bearer ${token}` };
}

export async function register(app: FastifyInstance, email: string, password = 'correct horse') {
  return app.inject({ method: 'POST', url: '/auth/register', payload: { email, password } });
}

export async function login(app: FastifyInstance, email: string, password = 'correct horse') {
  return app.inject({ method: 'POST', url: '/auth/login', payload: { email, password } });
}

/** Registers a user and returns a valid token. */
export async function signUp(app: FastifyInstance, email = 'alice@example.com') {
  await register(app, email);
  return (await login(app, email)).json<{ token: string }>().token;
}

export function createLink(
  app: FastifyInstance,
  token: string,
  payload: Record<string, unknown>,
): Promise<LightMyRequestResponse> {
  return app.inject({ method: 'POST', url: '/links', headers: bearer(token), payload });
}

/** The request logger runs after the response is sent, so wait for it. */
export async function waitForLogs(app: TestApp, count: number): Promise<void> {
  for (let i = 0; i < 200 && app.logs().length < count; i++) {
    await sleep(5);
  }
}
