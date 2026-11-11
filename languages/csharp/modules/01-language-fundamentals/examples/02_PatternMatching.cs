namespace M01.Examples;

/// <summary>Enums, switch expressions, patterns and nullable reference types.</summary>
static class PatternMatching
{
    enum TaskState
    {
        Todo,
        InProgress,
        Done,
    }

    abstract record Shape;

    sealed record Circle(double Radius) : Shape;

    sealed record Rect(double W, double H) : Shape;

    static string Label(TaskState s) => s switch
    {
        TaskState.Todo => "not started",
        TaskState.InProgress => "working",
        TaskState.Done => "finished",
        _ => throw new ArgumentOutOfRangeException(nameof(s)),
    };

    static string Describe(object? value) => value switch
    {
        null => "nothing",
        int and < 0 => "negative int",
        int n when n % 2 == 0 => $"even int {n}",
        int n => $"odd int {n}",
        string { Length: 0 } => "empty string",
        string s => $"string of {s.Length}",
        _ => value.GetType().Name,
    };

    static string Summarize(int[] numbers) => numbers switch
    {
        [] => "empty list",
        [var only] => $"list of one: {only}",
        [var first, .., var last] => $"list from {first} to {last}",
    };

    static double Area(Shape shape) => shape switch
    {
        Circle(var r) => Math.PI * r * r,
        Rect(var w, var h) => w * h,
        _ => throw new NotSupportedException(),
    };

    // Nullable reference types: the compiler tracks which values may be null.
    static string Greeting(string? name)
    {
        if (name is null or "")
        {
            return "Hello, stranger";
        }

        return $"Hello, {name.Trim()}"; // name is known non-null here
    }

    public static Task<int> RunAsync(string[] args)
    {
        foreach (var s in Enum.GetValues<TaskState>())
        {
            Console.WriteLine($"{s} -> {Label(s)}");
        }

        object?[] samples = [null, -4, 8, 7, "", "abc", 3.5];
        foreach (var sample in samples)
        {
            Console.WriteLine($"Describe: {Describe(sample)}");
        }

        int[][] lists = [[], [1], [1, 2, 3]];
        foreach (var list in lists)
        {
            Console.WriteLine($"Summarize: {Summarize(list)}");
        }

        Console.WriteLine($"circle area: {Area(new Circle(1)):F2}");
        Console.WriteLine($"rect area:   {Area(new Rect(2, 3)):F2}");

        string? missing = null;
        Console.WriteLine(Greeting(missing));
        Console.WriteLine(Greeting("  Ada "));
        Console.WriteLine($"length via ?. and ??: {missing?.Length ?? -1}");
        Console.WriteLine($"parsed enum: {Enum.Parse<TaskState>("done", ignoreCase: true)}");

        return Task.FromResult(0);
    }
}
