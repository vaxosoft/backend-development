using M07.Examples;

var examples = new Dictionary<string, Func<string[], Task<int>>>
{
    ["01-get-and-status"] = GetAndStatus.RunAsync,
    ["02-json-and-headers"] = JsonAndHeaders.RunAsync,
    ["03-typed-client"] = TypedClient.RunAsync,
    ["04-retry-and-timeout"] = RetryAndTimeout.RunAsync,
    ["05-task-tracker-client"] = TaskTrackerClientDemo.RunAsync,
};

if (args.Length == 0 || !examples.TryGetValue(args[0], out var run))
{
    Console.Error.WriteLine($"usage: dotnet run --project <this project> -- <{string.Join('|', examples.Keys)}>");
    return 2;
}

return await run(args[1..]);
