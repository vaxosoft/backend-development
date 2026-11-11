using M04.Examples;

var examples = new SortedDictionary<string, Func<string[], Task<int>>>
{
    ["json-basics"] = JsonBasics.RunAsync,
    ["nulls-and-unknowns"] = NullsAndUnknowns.RunAsync,
    ["custom-converter"] = CustomConverter.RunAsync,
    ["other-formats"] = OtherFormats.RunAsync,
    ["task-tracker"] = TaskTrackerJson.RunAsync,
};

if (args.Length == 0 || !examples.TryGetValue(args[0], out var run))
{
    Console.Error.WriteLine($"usage: dotnet run --project <project> -- <{string.Join('|', examples.Keys)}>");
    return 2;
}

return await run(args[1..]);
