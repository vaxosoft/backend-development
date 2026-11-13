export interface User {
  id: number;
  name: string;
  email: string;
}

export class ConflictError extends Error {
  constructor(message: string) {
    super(message);
    this.name = 'ConflictError';
  }
}

export class ValidationError extends Error {
  readonly fields: Record<string, string>;

  constructor(fields: Record<string, string>) {
    super('validation failed');
    this.name = 'ValidationError';
    this.fields = fields;
  }
}

/** The port the rest of the app depends on. Every implementation must satisfy the contract in the tests. */
export interface UserStore {
  /**
   * Ids start at 1 and are never reused, even after a delete. `name` is trimmed and must then be 1-100
   * characters, `email` is trimmed and must contain `@` (violations throw `ValidationError` whose
   * `fields` name the offending properties). Emails are unique ignoring case: a second user with
   * `ANN@x.io` when `ann@x.io` exists throws `ConflictError`. The stored email keeps its original case.
   */
  create(input: { name: string; email: string }): User;
  /** A copy of the user; changing the returned object must not change the store. */
  get(id: number): User | undefined;
  /** Case-insensitive lookup. */
  findByEmail(email: string): User | undefined;
  /** All users ordered by id ascending (copies). */
  list(): User[];
  /** Renames the user (same validation as `create`). Returns `undefined` for an unknown id. */
  rename(id: number, name: string): User | undefined;
  /** True when a user was removed. */
  delete(id: number): boolean;
}

export function createMemoryUserStore(): UserStore {
  throw new Error('TODO');
}
