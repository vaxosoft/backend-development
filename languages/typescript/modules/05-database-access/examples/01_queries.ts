// better-sqlite3 basics: prepared statements, bound parameters, and why never to concatenate SQL.
import Database from 'better-sqlite3';

const db = new Database(':memory:');
db.exec(`
  CREATE TABLE users (
    id    INTEGER PRIMARY KEY,
    name  TEXT NOT NULL,
    email TEXT NOT NULL UNIQUE,
    bio   TEXT
  )
`);

// prepare() once, run() many times. Positional (?) and named (@name) parameters are both supported.
const insert = db.prepare('INSERT INTO users (name, email, bio) VALUES (@name, @email, @bio)');
const first = insert.run({ name: 'Ada', email: 'ada@example.com', bio: null });
const second = insert.run({ name: 'Bob', email: 'bob@example.com', bio: 'Hi' });
console.log(
  'inserted ids:',
  first.lastInsertRowid,
  second.lastInsertRowid,
  'changes:',
  second.changes,
);

interface UserRow {
  id: number;
  name: string;
  email: string;
  bio: string | null;
}

// The Result type argument is only a promise to the compiler; the driver does not check it.
const byEmail = db.prepare<[string], UserRow>('SELECT * FROM users WHERE email = ?');
console.log(byEmail.get('ada@example.com'));
console.log(byEmail.get('nobody@example.com')); // get() returns undefined when there is no row

console.log(
  db
    .prepare<[], UserRow>('SELECT * FROM users ORDER BY id')
    .all()
    .map((u) => u.name),
);

// iterate() streams rows instead of building an array; pluck() returns the first column only.
for (const name of db.prepare('SELECT name FROM users ORDER BY name DESC').pluck().iterate()) {
  console.log('name:', name);
}

// Bound parameters are data, never SQL. Concatenation lets the input rewrite the query.
const evil = "x' OR '1'='1";
const unsafe = db.prepare(`SELECT count(*) AS n FROM users WHERE email = '${evil}'`).get();
const safe = db.prepare('SELECT count(*) AS n FROM users WHERE email = ?').get(evil);
console.log('concatenated:', unsafe, 'bound:', safe);

// Constraint violations are thrown as SqliteError with a stable `code`.
try {
  insert.run({ name: 'Ada again', email: 'ada@example.com', bio: null });
} catch (err: unknown) {
  if (err instanceof Database.SqliteError) console.log(err.code);
}

// Always release the file handle (and flush) when the process is done with the database.
db.close();
console.log('open after close:', db.open);
