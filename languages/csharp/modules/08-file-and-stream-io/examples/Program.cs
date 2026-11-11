using M08.Examples;

var examples = new Dictionary<string, Func<string[], Task<int>>>
{
    ["01-read-write-files"] = ReadWriteFiles.RunAsync,
    ["02-streams"] = Streams.RunAsync,
    ["03-paths-and-safety"] = PathsAndSafety.RunAsync,
    ["04-compress-encode-hash"] = CompressEncodeHash.RunAsync,
    ["05-task-archive"] = TaskArchiveDemo.RunAsync,
};

if (args.Length == 0 || !examples.TryGetValue(args[0], out var run))
{
    Console.Error.WriteLine($"usage: dotnet run --project <this project> -- <{string.Join('|', examples.Keys)}>");
    return 2;
}

return await run(args[1..]);
