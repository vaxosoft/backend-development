// What JSON.stringify / JSON.parse really do with values that are not plain data.

const task = {
  id: 1,
  title: 'Write docs',
  note: undefined,
  due: new Date('2030-01-31T00:00:00Z'),
  tags: new Set(['docs']),
  meta: new Map([['k', 'v']]),
  score: Number.NaN,
  fn: () => 1,
};

// undefined, functions and symbols vanish from objects; Set/Map become {}; NaN becomes null.
console.log(JSON.stringify(task));

// In arrays the same values become null so positions are preserved.
console.log(JSON.stringify([undefined, () => 1, Number.NaN, Number.POSITIVE_INFINITY]));

// bigint throws: JSON has no integer type wider than a double.
try {
  JSON.stringify({ big: 2n ** 63n });
} catch (err: unknown) {
  if (err instanceof TypeError) console.log(`TypeError: ${err.message}`);
}

// Numbers above 2^53 silently lose precision when parsed.
console.log(JSON.parse('{"id": 9007199254740993}').id);

// A replacer handles types JSON does not know; toJSON lets a class control its own encoding.
class Money {
  readonly cents: number;
  constructor(cents: number) {
    this.cents = cents;
  }
  toJSON(): string {
    return `${(this.cents / 100).toFixed(2)} EUR`;
  }
}

const text = JSON.stringify(
  { big: 2n ** 63n, set: new Set([1, 2]), price: new Money(1250) },
  (_key, value: unknown) => {
    if (typeof value === 'bigint') return value.toString();
    if (value instanceof Set) return [...value];
    return value;
  },
);
console.log(text);

// A reviver turns strings back into richer types while parsing.
const revived = JSON.parse('{"created_at":"2030-01-01T09:00:00Z","title":"x"}', (key, value) =>
  key === 'created_at' && typeof value === 'string' ? new Date(value) : value,
);
console.log(revived.created_at instanceof Date, revived.created_at.getUTCHours());

// JSON.parse returns `any`: assigning it to a typed variable checks nothing at runtime.
const wrong: { title: string } = JSON.parse('{"title": 42}');
console.log(typeof wrong.title);

// Malformed input throws SyntaxError.
try {
  JSON.parse('{"title": ');
} catch (err: unknown) {
  console.log(err instanceof SyntaxError ? 'SyntaxError' : 'other');
}
