// Types are erased at runtime, so untrusted JSON must be validated: zod gives one schema for both.
import { z } from 'zod';

const CreateUser = z.strictObject({
  name: z.string().trim().min(1).max(100),
  email: z.string().includes('@'),
  // optional: key may be absent. nullable: key may be null. They are different things.
  nickname: z.string().optional(),
  manager_id: z.number().int().nullable().default(null),
});

type CreateUser = z.infer<typeof CreateUser>;

function show(label: string, input: unknown): void {
  const result = CreateUser.safeParse(input);
  if (result.success) {
    console.log(`${label}: ok`, JSON.stringify(result.data));
    return;
  }
  for (const issue of result.error.issues) {
    console.log(`${label}: ${issue.path.join('.') || '(root)'} -> ${issue.message}`);
  }
}

show('valid', { name: ' Ada ', email: 'ada@example.com' });
show('null manager', { name: 'Bob', email: 'b@x.io', manager_id: null });
show('wrong types', { name: 42, email: 'nope', manager_id: 1.5 });
show('unknown key', { name: 'Eve', email: 'e@x.io', admin: true });
show('not an object', 'hello');

// z.infer derives the static type from the schema, so they cannot drift apart.
const user: CreateUser = CreateUser.parse({ name: 'Zed', email: 'z@x.io' });
console.log(user.manager_id, user.nickname);

// parse() throws a ZodError, safeParse() returns a discriminated union. Prefer safeParse in handlers.
try {
  CreateUser.parse({});
} catch (err: unknown) {
  if (err instanceof z.ZodError) console.log('ZodError issues:', err.issues.length);
}

// A transform changes the output type: the wire string becomes a Date.
const Event = z.object({ at: z.iso.datetime().transform((s) => new Date(s)) });
const event = Event.parse({ at: '2030-01-01T09:00:00Z' });
console.log(event.at.toISOString());

// z.input and z.output differ whenever defaults or transforms are used.
type EventIn = z.input<typeof Event>;
const wire: EventIn = { at: '2030-06-01T00:00:00Z' };
console.log(wire.at);
