import Database from 'better-sqlite3';

const db = new Database(':memory:');
db.exec(`
  CREATE TABLE tasks (id INTEGER PRIMARY KEY, owner_id INTEGER NOT NULL, title TEXT NOT NULL);
  INSERT INTO tasks (owner_id, title) VALUES (1, 'alice: pay rent'), (2, 'bob: secret plan');
`);

// Attacker-controlled value that is meant to be an owner id.
const hostile = '1 OR 1=1';

const concatenated = db.prepare(`SELECT title FROM tasks WHERE owner_id = ${hostile}`).all();
console.log('concatenated SQL leaks:', concatenated.length, 'rows');

// With a bound parameter the value stays data, whatever it contains.
const bound = db.prepare('SELECT title FROM tasks WHERE owner_id = ?').all(hostile);
console.log('bound parameter returns:', bound.length, 'rows');

const proper = db.prepare('SELECT title FROM tasks WHERE owner_id = ?').all(1);
console.log('owner 1 sees:', proper);

// Identifiers (column names, ORDER BY) cannot be bound: allow-list them.
const SORTABLE = { title: 'title', newest: 'id DESC' } as const;
function listSorted(sort: string) {
  const column = SORTABLE[sort as keyof typeof SORTABLE];
  if (!column) throw new RangeError(`cannot sort by ${JSON.stringify(sort)}`);
  return db.prepare(`SELECT title FROM tasks ORDER BY ${column}`).all();
}
console.log('sort=newest:', listSorted('newest'));
try {
  listSorted('title; DROP TABLE tasks');
} catch (err) {
  console.log('rejected:', (err as Error).message);
}

// XSS: encode untrusted text for the context it lands in (here: HTML text and attributes).
const HTML_ESCAPES: Record<string, string> = {
  '&': '&amp;',
  '<': '&lt;',
  '>': '&gt;',
  '"': '&quot;',
  "'": '&#39;',
};
const escapeHtml = (text: string) => text.replace(/[&<>"']/g, (ch) => HTML_ESCAPES[ch] ?? ch);

const title = '<img src=x onerror="alert(1)">';
console.log('raw :', `<li>${title}</li>`);
console.log('safe:', `<li>${escapeHtml(title)}</li>`);

db.close();
