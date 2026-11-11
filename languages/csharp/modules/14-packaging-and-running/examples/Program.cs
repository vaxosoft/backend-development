using M14.Examples;

var examples = new SortedDictionary<string, Func<string[], Task<int>>>
{
    ["01-build-info"] = BuildInfoDemo.RunAsync,
    ["02-runtime-flags"] = RuntimeFlags.RunAsync,
    ["03-json-source-gen"] = JsonSourceGen.RunAsync,
    ["04-task-service"] = TaskService.RunAsync,
};

if (args.Length == 0 || !examples.TryGetValue(args[0], out var run))
{
    Console.Error.WriteLine($"usage: dotnet run --project <project> -- <{string.Join('|', examples.Keys)}>");
    return 2;
}

return await run(args[1..]);
