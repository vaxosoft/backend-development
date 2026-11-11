using M12.Examples;

// An explicit entry class keeps the global `Program` name free for WebApplicationFactory<Program>
// (the app under test already exposes one).
internal static class ExampleRunner
{
    private static readonly SortedDictionary<string, Func<string[], Task<int>>> Examples = new()
    {
        ["01-fake-time"] = FakeTime.RunAsync,
        ["02-substitutes"] = Substitutes.RunAsync,
        ["03-sqlite-isolation"] = SqliteIsolation.RunAsync,
        ["04-benchmark"] = Benchmarks.RunAsync,
        ["05-in-memory-http"] = InMemoryHttp.RunAsync,
    };

    public static async Task<int> Main(string[] args)
    {
        if (args.Length == 0 || !Examples.TryGetValue(args[0], out var run))
        {
            Console.Error.WriteLine($"usage: dotnet run -c Release --project <project> -- <{string.Join('|', Examples.Keys)}>");
            return 2;
        }

        return await run(args[1..]);
    }
}
