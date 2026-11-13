// Transactions: all-or-nothing writes with db.transaction(), nesting, and the async trap.
import Database from 'better-sqlite3';

const db = new Database(':memory:');
db.pragma('foreign_keys = ON'); // SQLite ignores foreign keys unless this is set on every connection
db.exec(`
  CREATE TABLE accounts (
    id      INTEGER PRIMARY KEY,
    owner   TEXT NOT NULL,
    balance INTEGER NOT NULL CHECK (balance >= 0)
  );
  INSERT INTO accounts (owner, balance) VALUES ('ada', 100), ('bob', 20);
`);

const balances = () =>
  db.prepare('SELECT owner, balance FROM accounts ORDER BY id').all() as {
    owner: string;
    balance: number;
  }[];

const debit = db.prepare('UPDATE accounts SET balance = balance - ? WHERE owner = ?');
const credit = db.prepare('UPDATE accounts SET balance = balance + ? WHERE owner = ?');

// The wrapper runs BEGIN, the body, then COMMIT. Any exception becomes ROLLBACK and is rethrown.
const transfer = db.transaction((from: string, to: string, amount: number) => {
  debit.run(amount, from);
  credit.run(amount, to); // runs second, so a failed debit never reaches it
  return amount;
});

console.log('moved', transfer('ada', 'bob', 30), balances());

try {
  transfer('bob', 'ada', 500); // the CHECK constraint rejects the debit
} catch (err: unknown) {
  if (err instanceof Database.SqliteError) console.log('rolled back:', err.code);
}
console.log(balances());

// Without a transaction the first statement is already committed when the second one fails.
try {
  debit.run(10, 'ada');
  throw new Error('crash between the two statements');
} catch (err: unknown) {
  if (err instanceof Error) console.log(err.message);
}
console.log('money vanished:', balances());

// Calling a transaction function inside another one becomes a SAVEPOINT: an inner failure that
// the outer code catches only undoes the inner work.
const outer = db.transaction(() => {
  credit.run(1, 'ada');
  try {
    transfer('bob', 'ada', 9999);
  } catch {
    console.log('inner failed, outer continues');
  }
  credit.run(1, 'bob');
});
outer();
console.log(balances());

// Transaction bodies are synchronous: an async callback would commit before its awaits finish.
try {
  const broken = db.transaction(async () => {
    await Promise.resolve();
  });
  broken();
} catch (err: unknown) {
  if (err instanceof TypeError) console.log('TypeError:', err.message);
}

// .immediate takes the write lock up front, which avoids SQLITE_BUSY upgrades between processes.
db.transaction(() => credit.run(5, 'bob')).immediate();
console.log(balances(), 'inTransaction:', db.inTransaction);

db.close();
