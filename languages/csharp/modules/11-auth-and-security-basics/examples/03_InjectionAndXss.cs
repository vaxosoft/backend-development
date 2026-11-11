using System.Net;
using Microsoft.Data.Sqlite;

namespace M11.Examples;

public static class InjectionAndXss
{
    private static List<string> Query(SqliteConnection db, string sql, string? parameter = null)
    {
        using var command = db.CreateCommand();
        command.CommandText = sql;
        if (parameter is not null)
        {
            command.Parameters.AddWithValue("@email", parameter);
        }

        using var reader = command.ExecuteReader();
        var found = new List<string>();
        while (reader.Read())
        {
            found.Add(reader.GetString(0));
        }

        return found;
    }

    public static Task<int> RunAsync(string[] args)
    {
        using var db = new SqliteConnection("Data Source=:memory:");
        db.Open();
        using (var setup = db.CreateCommand())
        {
            setup.CommandText = """
                CREATE TABLE users (email TEXT PRIMARY KEY, name TEXT);
                INSERT INTO users VALUES ('ada@example.com', 'Ada'), ('bob@example.com', 'Bob');
                """;
            setup.ExecuteNonQuery();
        }

        var attack = "nobody@example.com' OR '1'='1";

        var concatenated = Query(db, $"SELECT name FROM users WHERE email = '{attack}'");
        Console.WriteLine($"concatenated SQL leaks: {string.Join(", ", concatenated)}");

        var parameterized = Query(db, "SELECT name FROM users WHERE email = @email", attack);
        Console.WriteLine($"parameterized rows: {parameterized.Count}");

        var comment = "<script>alert('xss')</script>";
        Console.WriteLine($"encoded output: {WebUtility.HtmlEncode(comment)}");
        return Task.FromResult(0);
    }
}
