using M03.Examples;

var examples = new SortedDictionary<string, Func<string[], Task<int>>>
{
    ["01-resources"] = Resources.RunAsync,
    ["02-validation"] = Validation.RunAsync,
    ["03-pagination"] = Pagination.RunAsync,
    ["04-versioning-idempotency"] = VersioningIdempotency.RunAsync,
    ["05-task-tracker-api"] = TaskTrackerApi.RunAsync,
};

if (args.Length == 0 || !examples.TryGetValue(args[0], out var run))
{
    Console.Error.WriteLine($"usage: ADDR=127.0.0.1:8080 dotnet run --project <examples> -- <{string.Join('|', examples.Keys)}>");
    return 2;
}

return await run(args[1..]);
