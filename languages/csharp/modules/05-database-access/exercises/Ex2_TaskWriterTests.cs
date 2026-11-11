using Microsoft.Data.Sqlite;

namespace M05.Exercises;

public class Ex2_TaskWriterTests
{
    private const string Tasks = "CREATE TABLE tasks (id INTEGER PRIMARY KEY, title TEXT NOT NULL CHECK (length(title) > 0))";
    private const string Tags = "CREATE TABLE task_tags (task_id INTEGER NOT NULL REFERENCES tasks(id), tag TEXT NOT NULL, PRIMARY KEY (task_id, tag))";

    [Fact]
    public async Task Create_inserts_task_and_tags()
    {
        await using var conn = TestDb.Open(Tasks, Tags);

        var id = await Ex2_TaskWriter.CreateWithTagsAsync(conn, "Write docs", ["docs", "p1"]);

        Assert.Equal(1, id);
        Assert.Equal(2, TestDb.Scalar(conn, "SELECT count(*) FROM task_tags WHERE task_id = 1"));
    }

    [Fact]
    public async Task Duplicate_tag_rolls_back_the_task_too()
    {
        await using var conn = TestDb.Open(Tasks, Tags);

        await Assert.ThrowsAsync<SqliteException>(() => Ex2_TaskWriter.CreateWithTagsAsync(conn, "Broken", ["a", "b", "a"]));

        Assert.Equal(0, TestDb.Scalar(conn, "SELECT count(*) FROM tasks"));
        Assert.Equal(0, TestDb.Scalar(conn, "SELECT count(*) FROM task_tags"));
    }

    [Fact]
    public async Task Failed_insert_does_not_poison_the_connection()
    {
        await using var conn = TestDb.Open(Tasks, Tags);

        await Assert.ThrowsAsync<SqliteException>(() => Ex2_TaskWriter.CreateWithTagsAsync(conn, "", ["a"]));
        var id = await Ex2_TaskWriter.CreateWithTagsAsync(conn, "Fine", ["a"]);

        Assert.Equal(1, TestDb.Scalar(conn, "SELECT count(*) FROM tasks"));
        Assert.Equal(id, TestDb.Scalar(conn, "SELECT id FROM tasks"));
    }

    [Fact]
    public async Task Replace_swaps_the_tag_set()
    {
        await using var conn = TestDb.Open(Tasks, Tags);
        var id = await Ex2_TaskWriter.CreateWithTagsAsync(conn, "T", ["old1", "old2"]);

        await Ex2_TaskWriter.ReplaceTagsAsync(conn, id, ["new"]);

        Assert.Equal(1, TestDb.Scalar(conn, "SELECT count(*) FROM task_tags"));
        Assert.Equal(1, TestDb.Scalar(conn, "SELECT count(*) FROM task_tags WHERE tag = 'new'"));
    }

    [Fact]
    public async Task Failed_replace_keeps_the_old_tags()
    {
        await using var conn = TestDb.Open(Tasks, Tags);
        var id = await Ex2_TaskWriter.CreateWithTagsAsync(conn, "T", ["old1", "old2"]);

        await Assert.ThrowsAsync<SqliteException>(() => Ex2_TaskWriter.ReplaceTagsAsync(conn, id, ["x", "x"]));

        Assert.Equal(2, TestDb.Scalar(conn, "SELECT count(*) FROM task_tags WHERE tag LIKE 'old%'"));
        Assert.Equal(2, TestDb.Scalar(conn, "SELECT count(*) FROM task_tags"));
    }
}
