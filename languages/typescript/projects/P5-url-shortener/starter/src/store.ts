import type Database from 'better-sqlite3';

export interface User {
  id: number;
  email: string;
  passwordHash: string;
}

export interface Link {
  slug: string;
  url: string;
  userId: number;
  hits: number;
  createdAt: string;
}

/**
 * TODO(P5): the repository. Every method is a prepared statement over `this.db`
 * (the schema is in db.ts). Never build SQL by concatenating input.
 */
export class Store {
  readonly db: Database.Database;

  constructor(db: Database.Database) {
    this.db = db;
  }

  /** Returns null when the email is already registered. */
  createUser(_email: string, _passwordHash: string): User | null {
    throw new Error('TODO');
  }

  findUserByEmail(_email: string): User | undefined {
    throw new Error('TODO');
  }

  /** Returns false when the slug is already taken. */
  createLink(_slug: string, _url: string, _userId: number): boolean {
    throw new Error('TODO');
  }

  findLink(_slug: string): Link | undefined {
    throw new Error('TODO');
  }

  /** Counts one visit and returns the target URL, or null for an unknown slug. */
  hit(_slug: string): string | null {
    throw new Error('TODO');
  }

  /** The user's own links, oldest first, plus the total number of their links. */
  listLinks(_userId: number, _limit: number, _offset: number): { items: Link[]; total: number } {
    throw new Error('TODO');
  }

  deleteLink(_slug: string): void {
    throw new Error('TODO');
  }
}
