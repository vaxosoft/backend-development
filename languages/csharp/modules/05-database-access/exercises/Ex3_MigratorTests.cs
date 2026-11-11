using Microsoft.Data.Sqlite;

namespace M05.Exercises;

public class Ex3_MigratorTests
{
    private static readonly Migration CreateTasks = new(1, "create tasks", "CREATE TABLE tasks (id INTEGER PRIMARY KEY, title TEXT NOT NULL)");
    private static readonly Migration AddStatus = new(2, "add status", "ALTER TABLE tasks ADD COLUMN status TEXT NOT NULL DEFAULT 'todo'");
    private static readonly Migration CreateTags = new(3, "tags", "CREATE TABLE task_tags (task_id INTEGER NOT NULL, tag TEXT NOT NULL); CREATE INDEX ix_tags ON task_tags (tag)");

    [Fact]
    public async Task Applies_all_pending_migrations_in_version_order()
    {
        await using var conn = TestDb.Open();

        var applied = await Ex3_Migrator.MigrateAsync(conn, [CreateTags, AddStatus, CreateTasks]);

        Assert.Equal([1, 2, 3], applied);
        Assert.Equal(3, await Ex3_Migrator.CurrentVersionAsync(conn));
        Assert.Equal(1, TestDb.Scalar(conn, "SELECT count(*) FROM pragma_table_info('tasks') WHERE name = 'status'"));
        Assert.Equal(1, TestDb.Scalar(conn, "SELECT count(*) FROM sqlite_master WHERE name = 'ix_tags'"));
    }

    [Fact]
    public async Task Current_version_is_zero_on_a_fresh_database()
    {
        await using var conn = TestDb.Open();

        Assert.Equal(0, await Ex3_Migrator.CurrentVersionAsync(conn));
    }

    [Fact]
    public async Task Second_run_is_a_no_op_and_new_migrations_are_picked_up()
    {
        await using var conn = TestDb.Open();
        await Ex3_Migrator.MigrateAsync(conn, [CreateTasks]);
        TestDb.Exec(conn, "INSERT INTO tasks (title) VALUES ('kept')");

        Assert.Empty(await Ex3_Migrator.MigrateAsync(conn, [CreateTasks]));
        Assert.Equal([2], await Ex3_Migrator.MigrateAsync(conn, [CreateTasks, AddStatus]));
        Assert.Equal(1, TestDb.Scalar(conn, "SELECT count(*) FROM tasks WHERE status = 'todo'"));
    }

    [Fact]
    public async Task Failing_migration_rolls_back_only_itself()
    {
        await using var conn = TestDb.Open();
        var broken = new Migration(2, "broken", "CREATE TABLE half_done (id INTEGER); INSERT INTO no_such_table VALUES (1)");

        await Assert.ThrowsAsync<SqliteException>(() => Ex3_Migrator.MigrateAsync(conn, [CreateTasks, broken, CreateTags]));

        Assert.Equal(1, await Ex3_Migrator.CurrentVersionAsync(conn));
        Assert.Equal(0, TestDb.Scalar(conn, "SELECT count(*) FROM sqlite_master WHERE name = 'half_done'"));
        Assert.Equal(0, TestDb.Scalar(conn, "SELECT count(*) FROM sqlite_master WHERE name = 'task_tags'"));
    }

    [Fact]
    public async Task Duplicate_versions_are_rejected_before_anything_runs()
    {
        await using var conn = TestDb.Open();

        await Assert.ThrowsAsync<ArgumentException>(() => Ex3_Migrator.MigrateAsync(conn, [CreateTasks, CreateTasks with { Name = "again" }]));

        Assert.Equal(0, TestDb.Scalar(conn, "SELECT count(*) FROM sqlite_master WHERE name = 'tasks'"));
    }

    [Fact]
    public async Task A_new_migration_older_than_the_latest_applied_is_an_error()
    {
        await using var conn = TestDb.Open();
        await Ex3_Migrator.MigrateAsync(conn, [CreateTasks, CreateTags]);

        await Assert.ThrowsAsync<InvalidOperationException>(() => Ex3_Migrator.MigrateAsync(conn, [CreateTasks, AddStatus, CreateTags]));
    }
}
