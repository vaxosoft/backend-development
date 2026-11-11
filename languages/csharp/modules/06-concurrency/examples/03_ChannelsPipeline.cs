using System.Threading.Channels;

namespace M06.Examples;

/// <summary>Channel&lt;T&gt;: bounded queue with back-pressure, fan-out to workers, fan-in to one reader, a staged pipeline.</summary>
public static class ChannelsPipeline
{
    public static async Task<int> RunAsync(string[] args)
    {
        // Stage 1: producer -> bounded channel (WriteAsync waits when it is full).
        var numbers = Channel.CreateBounded<int>(new BoundedChannelOptions(4) { SingleWriter = true });
        // Stage 2: 3 workers read from the same channel (fan-out) and write to one results channel (fan-in).
        var squares = Channel.CreateUnbounded<int>();

        var producer = Task.Run(async () =>
        {
            for (var i = 1; i <= 10; i++)
            {
                await numbers.Writer.WriteAsync(i);
            }

            numbers.Writer.Complete();
        });

        var workers = Enumerable.Range(1, 3).Select(_ => Task.Run(async () =>
        {
            await foreach (var n in numbers.Reader.ReadAllAsync())
            {
                await squares.Writer.WriteAsync(n * n);
            }
        })).ToArray();

        // Close the results channel only after every worker is done, otherwise the reader never ends.
        var closer = Task.WhenAll(workers).ContinueWith(_ => squares.Writer.Complete());

        var results = new List<int>();
        await foreach (var square in squares.Reader.ReadAllAsync())
        {
            results.Add(square);
        }

        await Task.WhenAll(producer, closer);
        results.Sort();
        Console.WriteLine($"squares (sorted): {string.Join(", ", results)}");
        Console.WriteLine($"sum: {results.Sum()}");

        // Back-pressure: a full bounded channel refuses TryWrite instead of growing.
        var tiny = Channel.CreateBounded<string>(2);
        var accepted = new[] { "a", "b", "c" }.Select(s => tiny.Writer.TryWrite(s)).ToArray();
        Console.WriteLine($"TryWrite into capacity 2: {string.Join(", ", accepted)}");

        // A pipeline of stages is just channels chained with async iterators.
        var words = await Collect(Filter(Upper(Source(["write", "docs", "fix", "bug", "ship"])), w => w.Length > 3));
        Console.WriteLine($"pipeline: {string.Join(", ", words)}");
        return 0;
    }

    private static async IAsyncEnumerable<string> Source(string[] items)
    {
        foreach (var item in items)
        {
            await Task.Yield();
            yield return item;
        }
    }

    private static async IAsyncEnumerable<string> Upper(IAsyncEnumerable<string> input)
    {
        await foreach (var s in input)
        {
            yield return s.ToUpperInvariant();
        }
    }

    private static async IAsyncEnumerable<string> Filter(IAsyncEnumerable<string> input, Func<string, bool> keep)
    {
        await foreach (var s in input)
        {
            if (keep(s))
            {
                yield return s;
            }
        }
    }

    private static async Task<List<string>> Collect(IAsyncEnumerable<string> input)
    {
        var list = new List<string>();
        await foreach (var s in input)
        {
            list.Add(s);
        }

        return list;
    }
}
