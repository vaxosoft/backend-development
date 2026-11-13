// Exceptions with `cause`, discriminated-union results, and async error handling.
import { err, NotFoundError, ok, type Result, ValidationError } from './_shared.ts';

// 1. Custom errors keep the original failure in `cause`.
function parseConfig(text: string): unknown {
  try {
    return JSON.parse(text);
  } catch (error) {
    throw new Error('config is not valid JSON', { cause: error });
  }
}
try {
  parseConfig('{oops');
} catch (error) {
  if (error instanceof Error && error.cause instanceof Error) {
    console.log(error.message, '<-', error.cause.name);
  }
}

// 2. Expected outcomes are values: the compiler forces callers to look at `ok`.
function parsePort(text: string): Result<number, string> {
  const port = Number(text);
  if (!Number.isInteger(port) || port < 1 || port > 65535) {
    return err(`invalid port: ${JSON.stringify(text)}`);
  }
  return ok(port);
}
for (const text of ['8080', '', 'abc', '70000']) {
  const result = parsePort(text);
  console.log(result.ok ? `port ${result.value}` : result.error);
}

// 3. `catch` gives `unknown`: narrow with instanceof before using it.
function describeFailure(error: unknown): string {
  if (error instanceof ValidationError) return `validation: ${Object.keys(error.fields)}`;
  if (error instanceof NotFoundError) return `not found: ${error.message}`;
  if (error instanceof Error) return `error: ${error.message}`;
  return `non-error thrown: ${String(error)}`;
}

// 4. Async functions turn throws into rejected promises; `await` turns them back.
async function loadTask(id: number): Promise<{ id: number }> {
  await Promise.resolve();
  if (id !== 1) throw new NotFoundError('task', id);
  return { id };
}
for (const id of [1, 2]) {
  try {
    console.log('loaded', await loadTask(id));
  } catch (error) {
    console.log(describeFailure(error));
  }
}
console.log(describeFailure(new ValidationError({ title: 'required' })));
console.log(describeFailure('a string'));

// 5. Without `await` the rejection escapes the try/catch: always await or return the promise.
async function forgetfulCaller(): Promise<void> {
  try {
    const pending = loadTask(99); // missing await
    console.log('no error caught here');
    await pending.catch((error: unknown) => console.log('handled later:', describeFailure(error)));
  } finally {
    console.log('done');
  }
}
await forgetfulCaller();
