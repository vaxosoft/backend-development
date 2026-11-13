import assert from 'node:assert/strict';
import { performance } from 'node:perf_hooks';

// Two ways to collect the distinct tags of many tasks, e.g. for a tag cloud.
function distinctTagsNaive(tags: string[]): string[] {
  const out: string[] = [];
  for (const tag of tags) if (!out.includes(tag)) out.push(tag);
  return out;
}

function distinctTagsSet(tags: string[]): string[] {
  return [...new Set(tags)];
}

interface Result {
  name: string;
  medianMs: number;
}

/** Warms up (so the JIT has compiled the code), then reports the median of several timed rounds. */
function bench(name: string, fn: () => unknown, { warmup = 5, rounds = 15 } = {}): Result {
  for (let i = 0; i < warmup; i++) fn();
  const samples: number[] = [];
  for (let i = 0; i < rounds; i++) {
    const start = performance.now();
    fn();
    samples.push(performance.now() - start);
  }
  samples.sort((a, b) => a - b);
  return { name, medianMs: samples[Math.floor(samples.length / 2)] ?? Number.NaN };
}

// 6000 tags drawn from 3000 distinct values, in a fixed order (no randomness).
const tags = Array.from({ length: 6000 }, (_, i) => `tag-${(i * 7) % 3000}`);

// A benchmark that compares different answers is meaningless: check equivalence first.
assert.deepEqual(distinctTagsNaive(tags), distinctTagsSet(tags));

const naive = bench('naive (Array.includes)', () => distinctTagsNaive(tags));
const set = bench('Set', () => distinctTagsSet(tags));

console.log('distinct tags:', distinctTagsSet(tags).length);
console.log('cases measured:', [naive.name, set.name].join(' | '));
console.log('fastest:', [naive, set].reduce((a, b) => (a.medianMs <= b.medianMs ? a : b)).name);
console.log('Set is at least 5x faster:', naive.medianMs / set.medianMs >= 5);

// Numbers are machine-specific and change on every run; print them to look, never assert them in tests.
if (process.env.SHOW_TIMINGS) {
  for (const r of [naive, set]) console.log(`${r.name.padEnd(24)} ${r.medianMs.toFixed(3)} ms`);
}
