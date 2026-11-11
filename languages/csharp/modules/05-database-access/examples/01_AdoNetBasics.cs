using Microsoft.Data.Sqlite;

namespace M05.Examples;

/// <summary>Connection, command, parameters, reader; every disposable in a using.</summary>
public static class AdoNetBasics
{
    public static async Task<int> RunAsync(string[] args)
    {
        // A private in-memory database lives as long as this one open connection.
        await using var conn = new SqliteConnection("Data Source=:memory:");
        await conn.OpenAsync();

        await using (var create = conn.CreateCommand())
        {
            create.CommandText = "CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT NOT NULL, email TEXT NOT NULL UNIQUE)";
            await create.ExecuteNonQueryAsync();
        }

        foreach (var (name, email) in new[] { ("Ada", "ada@example.com"), ("Grace", "grace@example.com"), ("Bobby'; DROP TABLE users;--", "bobby@example.com") })
        {
            await using var insert = conn.CreateCommand();
            insert.CommandText = "INSERT INTO users (name, email) VALUES (@name, @email) RETURNING id";
            insert.Parameters.AddWithValue("@name", name);
            insert.Parameters.AddWithValue("@email", email);
            var id = (long)(await insert.ExecuteScalarAsync())!;
            Console.WriteLine($"inserted id={id}");
        }

        await using (var select = conn.CreateCommand())
        {
            select.CommandText = "SELECT id, name, email FROM users ORDER BY id";
            await using var reader = await select.ExecuteReaderAsync();
            while (await reader.ReadAsync())
            {
                Console.WriteLine($"{reader.GetInt32(0)} | {reader.GetString(1)} | {reader.GetString(2)}");
            }
        }

        // The same lookup built two ways, fed hostile input.
        const string hostile = "x' OR '1'='1";
        await using (var bad = conn.CreateCommand())
        {
            bad.CommandText = $"SELECT count(*) FROM users WHERE email = '{hostile}'";
            Console.WriteLine($"concatenated: {await bad.ExecuteScalarAsync()} rows match (injection!)");
        }

        await using (var good = conn.CreateCommand())
        {
            good.CommandText = "SELECT count(*) FROM users WHERE email = @email";
            good.Parameters.AddWithValue("@email", hostile);
            Console.WriteLine($"parameterized: {await good.ExecuteScalarAsync()} rows match");
        }

        return 0;
    }
}
