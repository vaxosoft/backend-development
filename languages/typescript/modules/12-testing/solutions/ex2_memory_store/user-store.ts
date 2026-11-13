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

function cleanName(name: string): string {
  const trimmed = name.trim();
  if (trimmed.length < 1 || trimmed.length > 100) {
    throw new ValidationError({ name: 'must be 1-100 characters' });
  }
  return trimmed;
}

export function createMemoryUserStore(): UserStore {
  const users = new Map<number, User>();
  let nextId = 1;

  const findByEmail = (email: string) => {
    const wanted = email.trim().toLowerCase();
    return [...users.values()].find((u) => u.email.toLowerCase() === wanted);
  };

  return {
    create({ name, email }) {
      const fields: Record<string, string> = {};
      let cleanedName = '';
      try {
        cleanedName = cleanName(name);
      } catch (err) {
        if (!(err instanceof ValidationError)) throw err;
        Object.assign(fields, err.fields);
      }
      const cleanedEmail = email.trim();
      if (!cleanedEmail.includes('@')) fields.email = 'must contain @';
      if (Object.keys(fields).length > 0) throw new ValidationError(fields);
      if (findByEmail(cleanedEmail)) throw new ConflictError('email already registered');

      const user = { id: nextId++, name: cleanedName, email: cleanedEmail };
      users.set(user.id, user);
      return { ...user };
    },

    get(id) {
      const user = users.get(id);
      return user && { ...user };
    },

    findByEmail(email) {
      const user = findByEmail(email);
      return user && { ...user };
    },

    list() {
      return [...users.values()].sort((a, b) => a.id - b.id).map((u) => ({ ...u }));
    },

    rename(id, name) {
      const user = users.get(id);
      if (!user) return undefined;
      user.name = cleanName(name);
      return { ...user };
    },

    delete(id) {
      return users.delete(id);
    },
  };
}
