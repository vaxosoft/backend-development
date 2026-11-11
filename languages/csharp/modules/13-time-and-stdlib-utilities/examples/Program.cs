using M13.Examples;

var examples = new SortedDictionary<string, Func<string[], Task<int>>>
{
    ["01-dates-and-zones"] = DatesAndZones.RunAsync,
    ["02-timers"] = Timers.RunAsync,
    ["03-ids-random-hash"] = IdsRandomHash.RunAsync,
    ["04-regex-and-templates"] = RegexAndTemplates.RunAsync,
    ["05-reminder-worker"] = ReminderWorkerDemo.RunAsync,
};

if (args.Length == 0 || !examples.TryGetValue(args[0], out var run))
{
    Console.Error.WriteLine($"usage: dotnet run --project <project> -- <{string.Join('|', examples.Keys)}>");
    return 2;
}

return await run(args[1..]);
