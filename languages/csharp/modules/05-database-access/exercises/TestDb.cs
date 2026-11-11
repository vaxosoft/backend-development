using Microsoft.Data.Sqlite;

namespace M05.Exercises;

/// <summary>Each test gets its own private in-memory database.</summary>
public static class TestDb
{
    public static SqliteConnection Open(params string[] setupSql)
    {
        var conn = new SqliteConnection("Data Source=:memory:");
        conn.Open();
        Exec(conn, "PRAGMA foreign_keys = ON");
        foreach (var sql in setupSql)
        {
            Exec(conn, sql);
        }

        return conn;
    }

    public static void Exec(SqliteConnection conn, string sql)
    {
        using var cmd = conn.CreateCommand();
        cmd.CommandText = sql;
        cmd.ExecuteNonQuery();
    }

    public static long Scalar(SqliteConnection conn, string sql)
    {
        using var cmd = conn.CreateCommand();
        cmd.CommandText = sql;
        return Convert.ToInt64(cmd.ExecuteScalar());
    }
}
