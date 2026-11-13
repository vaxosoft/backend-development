import { z } from 'zod';

const email = z
  .string()
  .trim()
  .toLowerCase()
  .refine((value) => value.includes('@'), 'must be a valid email address');

export const registerSchema = z.object({
  email,
  password: z.string().min(8, 'must be at least 8 characters'),
});

export const loginSchema = z.object({ email, password: z.string().min(1, 'is required') });

function isHttpUrl(value: string): boolean {
  try {
    const { protocol } = new URL(value);
    return protocol === 'http:' || protocol === 'https:';
  } catch {
    return false;
  }
}

export const createLinkSchema = z.object({
  url: z.string().refine(isHttpUrl, 'must be an absolute http or https URL'),
  slug: z
    .string()
    .regex(/^[A-Za-z0-9_-]{3,32}$/, 'must be 3-32 characters of A-Z a-z 0-9 _ -')
    .optional(),
});
