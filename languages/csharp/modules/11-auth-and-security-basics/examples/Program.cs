using M11.Examples;

var examples = new SortedDictionary<string, Func<string[], Task<int>>>
{
    ["01-password-hashing"] = PasswordHashing.RunAsync,
    ["02-jwt"] = Jwt.RunAsync,
    ["03-injection-and-xss"] = InjectionAndXss.RunAsync,
    ["04-rate-limiter"] = RateLimiterDemo.RunAsync,
    ["05-tracker-auth-api"] = TrackerAuthApi.RunAsync,
};

if (args.Length == 0 || !examples.TryGetValue(args[0], out var run))
{
    Console.Error.WriteLine($"usage: dotnet run --project <project> -- <{string.Join('|', examples.Keys)}>");
    return 2;
}

return await run(args[1..]);
