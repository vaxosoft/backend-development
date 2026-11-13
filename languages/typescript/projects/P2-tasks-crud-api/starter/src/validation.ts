import type { ListQuery, Result, TaskInput, TaskPatch } from './types.ts';

/** Validates the body of POST /tasks and PUT /tasks/{id} (defaults filled in). */
export function parseTaskInput(_body: unknown): Result<TaskInput> {
  throw new Error('TODO');
}

/** Validates the body of PATCH /tasks/{id}: only supplied keys are present in the result. */
export function parseTaskPatch(_body: unknown): Result<TaskPatch> {
  throw new Error('TODO');
}

/** Validates the query string of GET /tasks (defaults, clamping and type conversion included). */
export function parseListQuery(_query: unknown): Result<ListQuery> {
  throw new Error('TODO');
}
