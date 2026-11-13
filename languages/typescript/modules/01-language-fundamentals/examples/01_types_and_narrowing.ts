// Structural typing, unions, narrowing, `unknown` vs `any`, and `===` vs `==`.

type Status = 'todo' | 'in_progress' | 'done';

interface Named {
  readonly name: string;
}

// Structural typing: any object with a `name: string` fits, no `implements` needed.
const greet = (who: Named): string => `Hello, ${who.name}!`;
const user = { name: 'Ada', email: 'ada@example.com' };
console.log(greet(user));

// A discriminated union: `kind` tells the compiler which variant it is looking at.
type Shape = { kind: 'circle'; radius: number } | { kind: 'rect'; width: number; height: number };

function area(shape: Shape): number {
  switch (shape.kind) {
    case 'circle':
      return Math.PI * shape.radius ** 2;
    case 'rect':
      return shape.width * shape.height;
    default: {
      // Adding a new variant to `Shape` turns this line into a compile error.
      const unreachable: never = shape;
      throw new Error(`unhandled shape: ${JSON.stringify(unreachable)}`);
    }
  }
}
console.log(area({ kind: 'rect', width: 3, height: 4 }));

// `unknown` forces a check before use; `any` silently switches the checker off.
function describe(value: unknown): string {
  if (value === null) return 'null';
  if (typeof value === 'string') return `string of length ${value.length}`;
  if (typeof value === 'number') return Number.isNaN(value) ? 'NaN' : `number ${value}`;
  if (Array.isArray(value)) return `array of ${value.length}`;
  if (typeof value === 'object') return `object with keys ${Object.keys(value).join(',')}`;
  return typeof value;
}
for (const v of ['abc', 42, Number('abc'), null, [1, 2], { a: 1 }, undefined]) {
  console.log(describe(v));
}
console.log(typeof null); // the historical quirk: "object"

// A user-defined type guard turns a runtime check into a type.
const STATUSES = ['todo', 'in_progress', 'done'] as const;
function isStatus(value: unknown): value is Status {
  return typeof value === 'string' && (STATUSES as readonly string[]).includes(value);
}
const input: unknown = JSON.parse('"done"');
if (isStatus(input)) {
  const next: Status = input; // narrowed from unknown
  console.log(`valid status: ${next}`);
}

// Coercion surprises: use `===` and convert explicitly. `==` is only acceptable for `x == null`.
console.log(Number(''), Number(undefined), Number(null), Number('12px'));
console.log(Number.isNaN(Number('x')), null == undefined, null === undefined);

// `readonly` and `as const` freeze the shape at compile time (not at runtime).
const defaults = { status: 'todo', tags: ['inbox'] } as const satisfies {
  status: Status;
  tags: readonly string[];
};
console.log(defaults.status, defaults.tags.length);
