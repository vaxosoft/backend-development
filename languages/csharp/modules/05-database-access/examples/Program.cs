using M05.Examples;

var examples = new SortedDictionary<string, Func<string[], Task<int>>>
{
    ["adonet-basics"] = AdoNetBasics.RunAsync,
    ["nulls-and-mapping"] = NullsAndMapping.RunAsync,
    ["transactions"] = Transactions.RunAsync,
    ["efcore"] = EfCoreBasics.RunAsync,
    ["task-repository"] = TaskRepositoryDemo.RunAsync,
};

if (args.Length == 0 || !examples.TryGetValue(args[0], out var run))
{
    Console.Error.WriteLine($"usage: dotnet run --project <project> -- <{string.Join('|', examples.Keys)}>");
    return 2;
}

return await run(args[1..]);
