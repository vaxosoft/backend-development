import type Database from 'better-sqlite3';

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
export function insertUser(_db: Database.Database, _input: NewUser): User {
  throw new Error('TODO');
}

/** Exact, bound lookup. Returns null (not undefined) when there is no such user. */
export function findUserByEmail(_db: Database.Database, _email: string): User | null {
  throw new Error('TODO');
}

/**
 * Case-insensitive "contains" search on name, ordered by name then id.
 * `%` and `_` in the term are literal characters, not wildcards. Never concatenate the term into the SQL.
 */
export function searchUsersByName(_db: Database.Database, _term: string): User[] {
  throw new Error('TODO');
}
