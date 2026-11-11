using M02.Examples;

var examples = new SortedDictionary<string, Func<string[], Task<int>>>
{
    ["01-hello"] = Hello.RunAsync,
    ["02-middleware"] = MiddlewarePipeline.RunAsync,
    ["03-request-response"] = RequestResponse.RunAsync,
    ["04-cors-static-shutdown"] = CorsStaticShutdown.RunAsync,
    ["05-task-tracker-server"] = TaskTrackerServer.RunAsync,
};

if (args.Length == 0 || !examples.TryGetValue(args[0], out var run))
{
    Console.Error.WriteLine($"usage: ADDR=127.0.0.1:8080 dotnet run --project <examples> -- <{string.Join('|', examples.Keys)}>");
    return 2;
}

return await run(args[1..]);
