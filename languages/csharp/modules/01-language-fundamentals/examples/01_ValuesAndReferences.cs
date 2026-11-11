namespace M01.Examples;

/// <summary>Classes are references, structs are copies, records compare by value.</summary>
static class ValuesAndReferences
{
    struct MutablePoint
    {
        public int X;
    }

    sealed class Counter
    {
        public int Value;
    }

    record Money(decimal Amount, string Currency);

    readonly record struct Point(int X, int Y);

    public static Task<int> RunAsync(string[] args)
    {
        var p1 = new MutablePoint { X = 1 };
        var p2 = p1; // copies the whole struct
        p2.X = 99;
        Console.WriteLine($"struct copy:      p1.X={p1.X} p2.X={p2.X}");

        var c1 = new Counter();
        var c2 = c1; // copies the reference
        c2.Value = 5;
        Console.WriteLine($"class alias:      c1.Value={c1.Value}");

        var a = new Counter { Value = 1 };
        var b = new Counter { Value = 1 };
        Console.WriteLine($"class ==:         {a == b} (reference identity)");

        var m1 = new Money(10m, "EUR");
        var m2 = new Money(10m, "EUR");
        var m3 = m1 with { Amount = 12m };
        Console.WriteLine($"record ==:        {m1 == m2}, with-copy differs: {m1 != m3}");
        Console.WriteLine($"record ToString:  {m3}");

        var seen = new HashSet<Point> { new(1, 2), new(1, 2), new(2, 1) };
        Console.WriteLine($"record struct set: {seen.Count} distinct points");

        var s1 = "task";
        var s2 = string.Concat("ta", "sk".AsSpan());
        Console.WriteLine($"string ==:        {s1 == s2}, same object: {ReferenceEquals(s1, s2)}");

        var points = new List<MutablePoint> { new() { X = 1 } };
        var copy = points[0]; // list indexer returns a copy
        copy.X = 42;
        Console.WriteLine($"struct in list:   points[0].X={points[0].X} copy.X={copy.X}");

        return Task.FromResult(0);
    }
}
