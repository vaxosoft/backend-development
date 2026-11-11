using M01.Examples;

var examples = new SortedDictionary<string, Func<string[], Task<int>>>
{
    ["01-values"] = ValuesAndReferences.RunAsync,
    ["02-patterns"] = PatternMatching.RunAsync,
    ["03-errors"] = ErrorsAndDisposal.RunAsync,
    ["04-linq-generics"] = LinqAndGenerics.RunAsync,
    ["05-task-tracker"] = TaskTrackerDomain.RunAsync,
};

if (args.Length == 0 || !examples.TryGetValue(args[0], out var run))
{
    Console.Error.WriteLine($"usage: dotnet run --project <examples> -- <{string.Join('|', examples.Keys)}>");
    return 2;
}

return await run(args[1..]);
