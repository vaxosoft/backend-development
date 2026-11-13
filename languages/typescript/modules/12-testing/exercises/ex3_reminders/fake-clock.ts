/** The only way `startReminders` may observe or schedule time. */
export interface Clock {
  now(): number;
  /** Runs `callback` every `ms` milliseconds; returns a function that cancels it. */
  setInterval(callback: () => void, ms: number): () => void;
}

/** Provided test helper: time moves only when `advance` is called; interval ticks run in order. */
export class FakeClock implements Clock {
  #now: number;
  #intervals: { id: number; ms: number; nextAt: number; callback: () => void }[] = [];
  #nextId = 1;

  constructor(start = 0) {
    this.#now = start;
  }

  now(): number {
    return this.#now;
  }

  setInterval(callback: () => void, ms: number): () => void {
    const id = this.#nextId++;
    this.#intervals.push({ id, ms, nextAt: this.#now + ms, callback });
    return () => {
      this.#intervals = this.#intervals.filter((i) => i.id !== id);
    };
  }

  advance(ms: number): void {
    const target = this.#now + ms;
    for (;;) {
      const due = this.#intervals
        .filter((i) => i.nextAt <= target)
        .sort((a, b) => a.nextAt - b.nextAt)[0];
      if (!due) break;
      this.#now = due.nextAt;
      due.nextAt += due.ms;
      due.callback();
    }
    this.#now = target;
  }
}
