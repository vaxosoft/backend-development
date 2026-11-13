// Runtime validation with zod: TypeScript types are erased, so request bodies must be checked.
import { type TaskInput, taskInputSchema, taskPatchSchema, zodFields } from './_shared.ts';

// 1. The static type comes from the schema (z.infer), so the two can never drift apart.
const parsed = taskInputSchema.parse({ title: '  Write docs  ', tags: ['docs', 'p1'] });
const typed: TaskInput = parsed;
console.log(typed);

// 2. safeParse never throws; map the issues to the `fields` object of the error envelope.
const bad: unknown = {
  title: '   ',
  status: 'finished',
  due_date: '2030-02-30',
  tags: ['Docs', 'a', 'a', 7],
  owner_id: 1.5,
};
const result = taskInputSchema.safeParse(bad);
if (!result.success) {
  console.log(zodFields(result.error));
}

// 3. Non-objects are rejected too: this is what `JSON.parse('null')` or `'[]'` would deliver.
for (const body of [null, 'text', []]) {
  const r = taskInputSchema.safeParse(body);
  console.log(JSON.stringify(body), r.success ? 'ok' : zodFields(r.error));
}

// 4. Unknown fields are stripped by default (use z.strictObject to reject them).
console.log(taskInputSchema.parse({ title: 'x', is_admin: true }));

// 5. PATCH: absent means "unchanged", null means "clear". The result keeps that distinction.
console.log(taskPatchSchema.parse({ title: 'New title' }));
console.log(taskPatchSchema.parse({ due_date: null }));
console.log(taskPatchSchema.safeParse({ status: 'nope' }).success);

// 6. Why types alone are not enough: this compiles, yet it is a lie at runtime.
const lie: TaskInput = JSON.parse('{"title":42}'); // `JSON.parse` returns `any`, which fits any type
console.log(typeof lie.title, '<- the compiler believes it is a string');
