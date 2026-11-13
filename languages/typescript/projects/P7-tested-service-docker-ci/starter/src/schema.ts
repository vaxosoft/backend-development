import { z } from 'zod';

const titleMessage = 'must be 1-200 characters';
const tag = z
  .string('must be a string')
  .min(1, 'must be 1-30 characters')
  .max(30, 'must be 1-30 characters')
  .refine((value) => value === value.toLowerCase(), 'must be lowercase');

export const newTaskSchema = z.object({
  title: z.string(titleMessage).trim().min(1, titleMessage).max(200, titleMessage),
  status: z.enum(['todo', 'in_progress', 'done']).default('todo'),
  due_date: z.iso.date('must be YYYY-MM-DD').nullable().default(null),
  tags: z
    .array(tag)
    .max(10, 'at most 10 tags')
    .refine((tags) => new Set(tags).size === tags.length, 'must not contain duplicates')
    .default([]),
  owner_id: z.int('must be an integer').nullable().default(null),
});

export const pageSchema = z.object({
  limit: z.coerce.number().int().min(0).default(20),
  offset: z.coerce.number().int().min(0).default(0),
});

export function fieldErrors(error: z.ZodError): Record<string, string> {
  const fields: Record<string, string> = {};
  for (const issue of error.issues) {
    fields[issue.path.join('.') || 'body'] ??= issue.message;
  }
  return fields;
}
