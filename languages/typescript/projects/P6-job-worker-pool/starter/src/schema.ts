import { z } from 'zod';

const reverse = z.object({
  type: z.literal('reverse'),
  payload: z.object({ text: z.string('must be a string').max(10_000, 'at most 10000 characters') }),
});

const ms = 'must be an integer from 0 to 5000';
const sleep = z.object({
  type: z.literal('sleep'),
  payload: z.object({ ms: z.int(ms).min(0, ms).max(5000, ms) }),
});

const fail = z.object({
  type: z.literal('fail'),
  payload: z.record(z.string(), z.unknown()).optional(),
});

/** One shape per job type: after `switch (request.type)` TypeScript knows the payload. */
export const jobRequestSchema = z.discriminatedUnion('type', [reverse, sleep, fail]);

export type JobRequest = z.infer<typeof jobRequestSchema>;

/** Flattens zod issues into `{ "payload.ms": "message" }` (first issue per path wins). */
export function fieldErrors(error: z.ZodError): Record<string, string> {
  const fields: Record<string, string> = {};
  for (const issue of error.issues) {
    const key = issue.path.join('.') || 'body';
    fields[key] ??= issue.message;
  }
  return fields;
}
