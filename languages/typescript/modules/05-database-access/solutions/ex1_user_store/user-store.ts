import Database from 'better-sqlite3';

export interface User {
  id: number;
  name: string;
  email: string;
  bio: string | null;
}

export interface NewUser {
  name: string;
  email: string;
  bio?: string | null;
}

export class EmailTakenError extends Error {
  readonly email: string;

  constructor(email: string, options?: ErrorOptions) {
    super(`email already registered: ${email}`, options);
    this.name = 'EmailTakenError';
    this.email = email;
  }
}

/** Inserts a user with a prepared statement and returns it with its generated id. A duplicate email throws EmailTakenError. */
export function insertUser(db: Database.Database, input: NewUser): User {
  try {
    const { lastInsertRowid } = db
      .prepare('INSERT INTO users (name, email, bio) VALUES (?, ?, ?)')
      .run(input.name, input.email, input.bio ?? null);
    return {
      id: Number(lastInsertRowid),
      name: input.name,
      email: input.email,
      bio: input.bio ?? null,
    };
  } catch (err: unknown) {
    if (err instanceof Database.SqliteError && err.code === 'SQLITE_CONSTRAINT_UNIQUE') {
      throw new EmailTakenError(input.email, { cause: err });
    }
    throw err;
  }
}

/** Exact, bound lookup. Returns null (not undefined) when there is no such user. */
export function findUserByEmail(db: Database.Database, email: string): User | null {
  const row = db
    .prepare<[string], User>('SELECT id, name, email, bio FROM users WHERE email = ?')
    .get(email);
  return row ?? null;
}

/**
 * Case-insensitive "contains" search on name, ordered by name then id.
 * `%` and `_` in the term are literal characters, not wildcards. Never concatenate the term into the SQL.
 */
export function searchUsersByName(db: Database.Database, term: string): User[] {
  const pattern = `%${term.replace(/[\\%_]/g, '\\$&')}%`;
  return db
    .prepare<[string], User>(
      "SELECT id, name, email, bio FROM users WHERE name LIKE ? ESCAPE '\\' ORDER BY name, id",
    )
    .all(pattern);
}
