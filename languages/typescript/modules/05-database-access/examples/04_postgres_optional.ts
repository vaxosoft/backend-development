// PostgreSQL with `pg`: the same ideas with a different driver. Runs only if DATABASE_URL is set,
// e.g. DATABASE_URL=postgres://user:pass@localhost:5432/tasks node modules/05-database-access/examples/04_postgres_optional.ts
import pg from 'pg';

const url = process.env.DATABASE_URL;

if (!url) {
  console.log('DATABASE_URL is not set: skipping the PostgreSQL example');
} else {
  // A Pool keeps connections open and hands them out; create one per process, not one per request.
  const pool = new pg.Pool({ connectionString: url, max: 4 });

  try {
    // Placeholders are $1, $2, ... and every call is asynchronous (unlike better-sqlite3).
    const { rows } = await pool.query<{ answer: number }>('SELECT $1::int AS answer', [42]);
    console.log('answer:', rows[0]?.answer);

    // A transaction must stay on ONE connection, so check a client out of the pool.
    const client = await pool.connect();
    try {
      await client.query('BEGIN');
      await client.query(
        'CREATE TEMP TABLE tasks_demo (id serial PRIMARY KEY, title text NOT NULL)',
      );
      await client.query('INSERT INTO tasks_demo (title) VALUES ($1)', ['Write docs']);
      const count = await client.query<{ n: string }>('SELECT count(*) AS n FROM tasks_demo');
      console.log('rows in transaction:', count.rows[0]?.n); // bigint columns arrive as strings
      await client.query('ROLLBACK');
    } catch (err: unknown) {
      await client.query('ROLLBACK');
      throw err;
    } finally {
      client.release(); // forgetting this leaks the connection and eventually hangs the pool
    }
  } finally {
    await pool.end();
  }
}
