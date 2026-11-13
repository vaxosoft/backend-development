// Map/Set, array methods, the default `sort`, iterators, generators and closures.

const tags = ['go', 'ts', 'rust', 'ts', 'go', 'c#'];

// Set removes duplicates and keeps insertion order.
console.log([...new Set(tags)]);

// Map keeps insertion order and accepts any key type.
const counts = new Map<string, number>();
for (const tag of tags) {
  counts.set(tag, (counts.get(tag) ?? 0) + 1);
}
console.log(counts);

// Pitfall: the default sort compares strings and sorts IN PLACE.
const nums = [10, 9, 1, 100];
console.log([...nums].sort());
console.log([...nums].sort((a, b) => a - b));
console.log(
  nums.toSorted((a, b) => b - a),
  nums,
); // toSorted returns a copy

// `noUncheckedIndexedAccess` makes `array[i]` possibly undefined.
const first = nums[0];
console.log(first === undefined ? 'empty' : first * 2);

// Value vs reference: assignment copies the reference, not the object.
const a = { n: 1 };
const b = a;
b.n = 2;
const c = { ...a }; // shallow copy
c.n = 3;
console.log(a.n, b.n, c.n);

// Generators are lazy iterators: values are produced on demand.
function* naturals(): Generator<number> {
  let n = 1;
  while (true) yield n++;
}
function* take<T>(source: Iterable<T>, count: number): Generator<T> {
  if (count <= 0) return;
  let seen = 0;
  for (const item of source) {
    yield item;
    if (++seen >= count) return;
  }
}
console.log([...take(naturals(), 5)]);

// Closures capture variables, not values.
function makeCounter(start = 0): { next: () => number; reset: () => void } {
  let current = start;
  return {
    next: () => ++current,
    reset: () => {
      current = start;
    },
  };
}
const counter = makeCounter(10);
console.log(counter.next(), counter.next());
counter.reset();
console.log(counter.next());

// `this` depends on how a method is called; arrow functions capture it lexically.
class Greeter {
  readonly #prefix: string;
  constructor(prefix: string) {
    this.#prefix = prefix;
  }
  greet(name: string): string {
    return `${this.#prefix}, ${name}`;
  }
  readonly greetBound = (name: string): string => this.greet(name);
}
const greeter = new Greeter('Hi');
const detached = greeter.greetBound; // safe to pass around
console.log(detached('Grace'));
const unbound = greeter.greet;
try {
  console.log(unbound('Grace'));
} catch (error) {
  console.log(error instanceof TypeError ? 'TypeError: lost `this`' : 'unexpected');
}
