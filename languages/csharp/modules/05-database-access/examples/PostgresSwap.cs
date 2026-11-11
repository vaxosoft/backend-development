using Npgsql;

namespace M05.Examples;

/// <summary>
/// The PostgreSQL swap: same ADO.NET shape, different provider types. Compiles always;
/// connects only when DATABASE_URL (postgres://user:pass@host:5432/db) is set.
/// </summary>
public static class PostgresSwap
{
    public static async Task TryAsync()
    {
        var url = Environment.GetEnvironmentVariable("DATABASE_URL");
        if (string.IsNullOrEmpty(url))
        {
            Console.WriteLine("PostgreSQL: DATABASE_URL not set, skipping");
            return;
        }

        await using var conn = new NpgsqlConnection(ToConnectionString(url));
        await conn.OpenAsync();
        await using var cmd = new NpgsqlCommand("SELECT version()", conn);
        Console.WriteLine($"PostgreSQL: {await cmd.ExecuteScalarAsync()}");
    }

    public static string ToConnectionString(string url)
    {
        var uri = new Uri(url);
        var userInfo = uri.UserInfo.Split(':', 2);
        return new NpgsqlConnectionStringBuilder
        {
            Host = uri.Host,
            Port = uri.IsDefaultPort ? 5432 : uri.Port,
            Database = uri.AbsolutePath.TrimStart('/'),
            Username = Uri.UnescapeDataString(userInfo[0]),
            Password = userInfo.Length > 1 ? Uri.UnescapeDataString(userInfo[1]) : null,
        }.ConnectionString;
    }
}
