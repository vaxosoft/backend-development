using M09.Examples;

var examples = new Dictionary<string, Func<string[], Task<int>>>
{
    ["01-layers"] = Layers.RunAsync,
    ["02-binding"] = Binding.RunAsync,
    ["03-validation"] = Validation.RunAsync,
    ["04-reload-and-lifetimes"] = ReloadAndLifetimes.RunAsync,
    ["05-task-tracker-config"] = TaskTrackerConfig.RunAsync,
};

if (args.Length == 0 || !examples.TryGetValue(args[0], out var run))
{
    Console.Error.WriteLine($"usage: dotnet run --project <this project> -- <{string.Join('|', examples.Keys)}>");
    return 2;
}

return await run(args[1..]);
