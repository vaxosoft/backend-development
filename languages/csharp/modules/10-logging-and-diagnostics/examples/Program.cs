using M10.Examples;

var examples = new Dictionary<string, Func<string[], Task<int>>>
{
    ["01-logger-basics"] = LoggerBasics.RunAsync,
    ["02-scopes-and-json"] = ScopesAndJson.RunAsync,
    ["03-exception-handling"] = ExceptionHandling.RunAsync,
    ["04-metrics-and-activities"] = MetricsAndActivities.RunAsync,
    ["05-task-tracker-diagnostics"] = TaskTrackerDiagnostics.RunAsync,
};

if (args.Length == 0 || !examples.TryGetValue(args[0], out var run))
{
    Console.Error.WriteLine($"usage: dotnet run --project <this project> -- <{string.Join('|', examples.Keys)}>");
    return 2;
}

return await run(args[1..]);
