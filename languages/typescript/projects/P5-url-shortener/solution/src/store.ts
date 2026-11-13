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

interface UserRow {
  id: number;
  email: string;
  password_hash: string;
}

interface LinkRow {
  slug: string;
  url: string;
  user_id: number;
  hits: number;
  created_at: string;
}

const toUser = (row: UserRow): User => ({
  id: row.id,
  email: row.email,
  passwordHash: row.password_hash,
});

const toLink = (row: LinkRow): Link => ({
  slug: row.slug,
  url: row.url,
  userId: row.user_id,
  hits: row.hits,
  createdAt: row.created_at,
});

export class Store {
  readonly db: Database.Database;

  constructor(db: Database.Database) {
    this.db = db;
  }

  /** Returns null when the email is already registered. */
  createUser(email: string, passwordHash: string): User | null {
    const result = this.db
      .prepare('INSERT INTO users (email, password_hash) VALUES (?, ?) ON CONFLICT DO NOTHING')
      .run(email, passwordHash);
    if (result.changes === 0) {
      return null;
    }
    return { id: Number(result.lastInsertRowid), email, passwordHash };
  }

  findUserByEmail(email: string): User | undefined {
    const row = this.db.prepare('SELECT * FROM users WHERE email = ?').get(email) as
      | UserRow
      | undefined;
    return row && toUser(row);
  }

  /** Returns false when the slug is already taken. */
  createLink(slug: string, url: string, userId: number): boolean {
    const result = this.db
      .prepare(
        'INSERT INTO links (slug, url, user_id, created_at) VALUES (?, ?, ?, ?) ON CONFLICT DO NOTHING',
      )
      .run(slug, url, userId, new Date().toISOString());
    return result.changes === 1;
  }

  findLink(slug: string): Link | undefined {
    const row = this.db.prepare('SELECT * FROM links WHERE slug = ?').get(slug) as
      | LinkRow
      | undefined;
    return row && toLink(row);
  }

  /** Counts one visit and returns the target URL, or null for an unknown slug. */
  hit(slug: string): string | null {
    const row = this.db
      .prepare('UPDATE links SET hits = hits + 1 WHERE slug = ? RETURNING url')
      .get(slug) as { url: string } | undefined;
    return row?.url ?? null;
  }

  /** The user's own links, oldest first, plus the total number of their links. */
  listLinks(userId: number, limit: number, offset: number): { items: Link[]; total: number } {
    const rows = this.db
      .prepare('SELECT * FROM links WHERE user_id = ? ORDER BY created_at, rowid LIMIT ? OFFSET ?')
      .all(userId, limit, offset) as LinkRow[];
    const count = this.db
      .prepare('SELECT COUNT(*) AS n FROM links WHERE user_id = ?')
      .get(userId) as {
      n: number;
    };
    return { items: rows.map(toLink), total: count.n };
  }

  deleteLink(slug: string): void {
    this.db.prepare('DELETE FROM links WHERE slug = ?').run(slug);
  }
}
