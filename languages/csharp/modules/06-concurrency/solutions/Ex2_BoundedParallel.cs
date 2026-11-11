namespace M06.Exercises;

public static class Ex2_BoundedParallel
{
    /// <summary>
    /// Run <paramref name="work"/> for every item with at most <paramref name="maxConcurrency"/> calls in flight
    /// and return the results in INPUT order.
    /// <list type="bullet">
    /// <item><paramref name="maxConcurrency"/> below 1: ArgumentOutOfRangeException.</item>
    /// <item>The first failing item stops new items from starting, cancels the token given to in-flight items,
    /// and its exception (not an AggregateException) is rethrown after all in-flight items have finished.</item>
    /// <item>Cancelling <paramref name="ct"/> surfaces as OperationCanceledException.</item>
    /// </list>
    /// </summary>
    public static async Task<TOut[]> MapAsync<TIn, TOut>(
        IReadOnlyList<TIn> items,
        int maxConcurrency,
        Func<TIn, CancellationToken, Task<TOut>> work,
        CancellationToken ct = default)
    {
        ArgumentOutOfRangeException.ThrowIfLessThan(maxConcurrency, 1);

        var results = new TOut[items.Count];
        await Parallel.ForEachAsync(
            Enumerable.Range(0, items.Count),
            new ParallelOptions { MaxDegreeOfParallelism = maxConcurrency, CancellationToken = ct },
            async (index, token) => results[index] = await work(items[index], token));
        return results;
    }
}
