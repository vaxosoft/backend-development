using M06.Examples;

var examples = new SortedDictionary<string, Func<string[], Task<int>>>
{
    ["async-basics"] = AsyncBasics.RunAsync,
    ["shared-state"] = SharedState.RunAsync,
    ["channels"] = ChannelsPipeline.RunAsync,
    ["cancellation"] = CancellationAndTimeouts.RunAsync,
    ["worker-pool"] = TrackerWorkerPool.RunAsync,
};

if (args.Length == 0 || !examples.TryGetValue(args[0], out var run))
{
    Console.Error.WriteLine($"usage: dotnet run --project <project> -- <{string.Join('|', examples.Keys)}>");
    return 2;
}

return await run(args[1..]);
