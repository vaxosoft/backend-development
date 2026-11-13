// Timeouts and cancellation: AbortSignal.timeout, AbortController and AbortSignal.any.
import { startServer } from './_shared.ts';

// A handler that never answers: the client must give up on its own.
const server = await startServer(() => {});

async function attempt(label: string, signal: AbortSignal): Promise<void> {
  try {
    await fetch(server.url, { signal });
  } catch (err) {
    if (err instanceof DOMException) {
      console.log(label, '->', err.name);
    } else {
      throw err;
    }
  }
}

try {
  await attempt('timeout', AbortSignal.timeout(50));

  const controller = new AbortController();
  setTimeout(() => controller.abort(), 10);
  await attempt('user abort', controller.signal);

  // Combine a per-attempt timeout with a caller-owned cancellation signal.
  const caller = new AbortController();
  const either = AbortSignal.any([caller.signal, AbortSignal.timeout(500)]);
  setTimeout(() => caller.abort(new Error('shutting down')), 10);
  try {
    await fetch(server.url, { signal: either });
  } catch (err) {
    console.log('abort reason ->', err instanceof Error ? err.message : err);
  }
} finally {
  await server.close();
}
