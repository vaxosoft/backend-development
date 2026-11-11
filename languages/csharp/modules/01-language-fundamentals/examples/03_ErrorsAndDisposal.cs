namespace M01.Examples;

/// <summary>Exceptions for exceptional cases, result values for expected failures, IDisposable.</summary>
static class ErrorsAndDisposal
{
    sealed class NotFoundException(string resource, int id) : Exception($"{resource} {id} not found")
    {
        public int Id { get; } = id;
    }

    // Expected failure as a value: no exception, the caller must look at the result.
    abstract record Result<T>
    {
        public sealed record Ok(T Value) : Result<T>;

        public sealed record Error(string Message) : Result<T>;
    }

    static Result<int> ParsePort(string? text) => int.TryParse(text, out var port) && port is > 0 and < 65536
        ? new Result<int>.Ok(port)
        : new Result<int>.Error($"'{text}' is not a valid port");

    sealed class Connection(string name) : IDisposable
    {
        public void Query() => Console.WriteLine($"  {name}: query");

        public void Dispose() => Console.WriteLine($"  {name}: disposed");
    }

    static void Find(int id) => throw new NotFoundException("task", id);

    public static Task<int> RunAsync(string[] args)
    {
        foreach (var text in new[] { "8080", "http", "70000", null })
        {
            var message = ParsePort(text) switch
            {
                Result<int>.Ok(var port) => $"port {port}",
                Result<int>.Error(var error) => $"error: {error}",
                _ => throw new InvalidOperationException(),
            };
            Console.WriteLine(message);
        }

        try
        {
            Find(42);
        }
        catch (NotFoundException ex) when (ex.Id > 100)
        {
            Console.WriteLine("not reached: filter rejects it");
        }
        catch (NotFoundException ex)
        {
            Console.WriteLine($"caught: {ex.Message}");
        }
        finally
        {
            Console.WriteLine("finally runs");
        }

        Console.WriteLine("using declaration:");
        try
        {
            using var db = new Connection("db");
            using var cache = new Connection("cache");
            db.Query();
            throw new InvalidOperationException("boom");
        }
        catch (InvalidOperationException ex)
        {
            Console.WriteLine($"after dispose (reverse order): {ex.Message}");
        }

        try
        {
            ArgumentNullException.ThrowIfNull(null as string, "name");
        }
        catch (ArgumentNullException ex)
        {
            Console.WriteLine($"guard clause: {ex.ParamName}");
        }

        return Task.FromResult(0);
    }
}
