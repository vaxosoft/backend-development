import type { JobRequest } from './schema.ts';

export type JobType = JobRequest['type'];

export type JobStatus = 'queued' | 'running' | 'succeeded' | 'failed' | 'cancelled';

/** The public view of a job: exactly what `GET /jobs/{id}` returns. */
export interface Job {
  id: number;
  type: JobType;
  status: JobStatus;
  attempts: number;
  result: string | null;
  error: string | null;
}

export function errorBody(code: string, message: string, fields?: Record<string, string>) {
  return { error: { code, message, ...(fields ? { fields } : {}) } };
}
