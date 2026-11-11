namespace M01.Examples;

/// <summary>LINQ and its deferred execution, generics with constraints, closures.</summary>
static class LinqAndGenerics
{
    interface IEntity<out TKey>
    {
        TKey Id { get; }
    }

    record Item(int Id, string Name) : IEntity<int>;

    static T Largest<T>(IEnumerable<T> source)
        where T : IComparable<T>
    {
        using var e = source.GetEnumerator();
        if (!e.MoveNext())
        {
            throw new InvalidOperationException("empty sequence");
        }

        var best = e.Current;
        while (e.MoveNext())
        {
            if (e.Current.CompareTo(best) > 0)
            {
                best = e.Current;
            }
        }

        return best;
    }

    static Dictionary<TKey, T> IndexById<TKey, T>(IEnumerable<T> items)
        where TKey : notnull
        where T : IEntity<TKey> => items.ToDictionary(i => i.Id);

    static IEnumerable<int> Numbers(List<string> log)
    {
        for (var i = 1; i <= 3; i++)
        {
            log.Add($"produce {i}");
            yield return i;
        }
    }

    public static Task<int> RunAsync(string[] args)
    {
        var log = new List<string>();
        var query = Numbers(log).Where(n => n % 2 == 1).Select(n => n * 10);
        Console.WriteLine($"after building the query: {log.Count} produced");

        var first = query.ToList();
        Console.WriteLine($"first enumeration:  [{string.Join(", ", first)}], produced {log.Count}");
        _ = query.Count();
        Console.WriteLine($"second enumeration: produced {log.Count} (the work was repeated)");

        Console.WriteLine($"Largest ints:    {Largest([3, 9, 4])}");
        Console.WriteLine($"Largest strings: {Largest(["pear", "apple", "zebra"])}");

        var index = IndexById<int, Item>([new(1, "write"), new(2, "test")]);
        Console.WriteLine($"index[2] = {index[2].Name}");

        var words = new[] { "api", "http", "json", "sql", "tcp" };
        foreach (var group in words.GroupBy(w => w.Length).OrderBy(g => g.Key))
        {
            Console.WriteLine($"length {group.Key}: {string.Join(", ", group)}");
        }

        var actions = new List<Func<int>>();
        for (var i = 0; i < 3; i++)
        {
            actions.Add(() => i); // one shared 'i' for the whole loop
        }

        var perIteration = new List<Func<int>>();
        foreach (var n in new[] { 0, 1, 2 })
        {
            perIteration.Add(() => n); // a fresh 'n' per iteration
        }

        Console.WriteLine($"for-loop closures:     {string.Join(",", actions.Select(a => a()))}");
        Console.WriteLine($"foreach-loop closures: {string.Join(",", perIteration.Select(a => a()))}");

        return Task.FromResult(0);
    }
}
