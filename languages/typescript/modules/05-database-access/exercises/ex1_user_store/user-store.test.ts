import assert from 'node:assert/strict';
import { afterEach, beforeEach, describe, test } from 'node:test';
import Database from 'better-sqlite3';
import { EmailTakenError, findUserByEmail, insertUser, searchUsersByName } from './user-store.ts';

let db: Database.Database;

beforeEach(() => {
  db = new Database(':memory:');
  db.exec(`CREATE TABLE users (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL,
    email TEXT NOT NULL UNIQUE,
    bio TEXT
  )`);
});

afterEach(() => {
  db.close();
});

describe('insertUser / findUserByEmail', () => {
  test('insert returns the stored user with an id and null bio by default', () => {
    const user = insertUser(db, { name: 'Ada', email: 'ada@example.com' });
    assert.deepEqual(user, { id: 1, name: 'Ada', email: 'ada@example.com', bio: null });
    assert.equal(insertUser(db, { name: 'Bob', email: 'bob@example.com', bio: 'Hi' }).id, 2);
  });

  test('findUserByEmail returns the user or null', () => {
    insertUser(db, { name: 'Ada', email: 'ada@example.com', bio: 'x' });
    assert.deepEqual(findUserByEmail(db, 'ada@example.com'), {
      id: 1,
      name: 'Ada',
      email: 'ada@example.com',
      bio: 'x',
    });
    assert.equal(findUserByEmail(db, 'nobody@example.com'), null);
  });

  test('a duplicate email becomes EmailTakenError and keeps the original cause', () => {
    insertUser(db, { name: 'Ada', email: 'ada@example.com' });
    assert.throws(
      () => insertUser(db, { name: 'Other', email: 'ada@example.com' }),
      (err: unknown) => {
        assert.ok(err instanceof EmailTakenError);
        assert.equal(err.email, 'ada@example.com');
        assert.ok(err.cause instanceof Database.SqliteError);
        return true;
      },
    );
  });

  test('other database errors are not disguised as EmailTakenError', () => {
    assert.throws(
      () => insertUser(db, { name: null as unknown as string, email: 'n@example.com' }),
      (err: unknown) => err instanceof Database.SqliteError && !(err instanceof EmailTakenError),
    );
  });

  test('hostile input is stored and matched as plain data', () => {
    const email = "x'); DROP TABLE users; --";
    insertUser(db, { name: "Robert'); DROP TABLE users;--", email });
    assert.equal(findUserByEmail(db, email)?.email, email);
    assert.equal(findUserByEmail(db, "' OR '1'='1"), null);
    assert.equal(db.prepare('SELECT count(*) FROM users').pluck().get(), 1);
  });
});

describe('searchUsersByName', () => {
  beforeEach(() => {
    ['Ada Lovelace', 'adam', 'Bob', '100% Real', 'a_b'].forEach((name, i) => {
      insertUser(db, { name, email: `user${i}@example.com` });
    });
  });

  const names = (term: string) => searchUsersByName(db, term).map((u) => u.name);

  test('matches substrings case-insensitively and orders by name', () => {
    assert.deepEqual(names('AD'), ['Ada Lovelace', 'adam']);
    assert.deepEqual(names('o'), ['Ada Lovelace', 'Bob']);
  });

  test('% and _ are literal', () => {
    assert.deepEqual(names('%'), ['100% Real']);
    assert.deepEqual(names('_'), ['a_b']);
  });

  test('an empty term matches everyone and no match gives an empty list', () => {
    assert.equal(names('').length, 5);
    assert.deepEqual(names('zzz'), []);
  });

  test('injection attempts find nothing and change nothing', () => {
    assert.deepEqual(names("' OR '1'='1"), []);
    assert.deepEqual(names("'; DROP TABLE users; --"), []);
    assert.equal(db.prepare('SELECT count(*) FROM users').pluck().get(), 5);
  });
});
