"""SQLAlchemy Core (query builder, no ORM) and the PostgreSQL swap, which needs DATABASE_URL."""

import os

import sqlalchemy as sa

metadata = sa.MetaData()
tasks = sa.Table(
    "tasks",
    metadata,
    sa.Column("id", sa.Integer, primary_key=True),
    sa.Column("title", sa.String(200), nullable=False),
    sa.Column("status", sa.String(20), nullable=False, server_default="todo"),
    sa.Column("owner_id", sa.Integer),
)

engine = sa.create_engine("sqlite://")  # in-memory SQLite
metadata.create_all(engine)

# engine.begin() is a transaction: commit on success, rollback on exception, connection returned.
with engine.begin() as conn:
    conn.execute(
        sa.insert(tasks),
        [
            {"title": "Write docs", "status": "todo", "owner_id": 1},
            {"title": "Review PR", "status": "in_progress", "owner_id": 1},
            {"title": "Deploy", "status": "done", "owner_id": None},
        ],
    )

# Expressions are Python objects: compose conditions, the values become bound parameters.
open_tasks = sa.select(tasks.c.id, tasks.c.title).where(
    tasks.c.status != "done", tasks.c.owner_id == 1
)
print(" ".join(str(open_tasks).split()))
with engine.connect() as conn:
    for row in conn.execute(open_tasks.order_by(tasks.c.id)):
        print(row.id, row.title)  # rows are named tuples

    per_status = sa.select(tasks.c.status, sa.func.count().label("n")).group_by(tasks.c.status)
    print(sorted(tuple(r) for r in conn.execute(per_status)))

# Hostile input is still just a bound value.
hostile = sa.select(tasks.c.id).where(tasks.c.title == "x'; DROP TABLE tasks; --")
with engine.connect() as conn:
    print(conn.execute(hostile).all(), "| tables:", sa.inspect(engine).get_table_names())

# The same expression compiles for PostgreSQL. create_engine is lazy: nothing connects here.
pg_engine = sa.create_engine("postgresql+psycopg://user:secret@localhost/tasks")
print(" ".join(str(open_tasks.compile(pg_engine)).split()))

# --- PostgreSQL: only runs when DATABASE_URL is set, e.g. postgresql://user:pw@localhost/tasks
url = os.environ.get("DATABASE_URL")
if url is None:
    print("DATABASE_URL not set: skipping the PostgreSQL section")
else:
    import psycopg

    # psycopg uses %s placeholders; the DB-API rules (parameters, transactions) are the same.
    with psycopg.connect(url) as pg, pg.cursor() as cur:  # the with block commits or rolls back
        cur.execute("SELECT %s::text", ("connected",))
        print(cur.fetchone())
