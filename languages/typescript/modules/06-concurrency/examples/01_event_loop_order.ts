// The event loop: synchronous code first, then microtasks, then timers and I/O callbacks.
import { readFile } from 'node:fs';

const log: string[] = [];
const note = (s: string) => log.push(s);

note('sync: start');

setTimeout(() => note('macrotask: setTimeout 0'), 0);

Promise.resolve().then(() => note('microtask: promise.then'));

queueMicrotask(() => note('microtask: queueMicrotask'));

// In an ES module the top level itself runs inside a promise job, so the microtask queue
// drains before process.nextTick callbacks. In a CommonJS script nextTick would come first.
process.nextTick(() => note('nextTick'));

(async () => {
  note('sync: async function body runs until its first await');
  await null;
  note('microtask: code after await');
})();

note('sync: end');

// Inside an I/O callback, setImmediate always runs before a 0 ms timer.
readFile(import.meta.filename, () => {
  setTimeout(() => note('timer inside I/O callback'), 0);
  setImmediate(() => note('setImmediate inside I/O callback'));
});

setTimeout(() => {
  for (const line of log) console.log(line);
}, 50);
