namespace M01.Exercises;

/// <summary>Either a value or a list of error messages.</summary>
public abstract record Result<T>
{
    /// <summary>Success.</summary>
    public sealed record Ok(T Value) : Result<T>;

    /// <summary>Failure with at least one message.</summary>
    public sealed record Failure(IReadOnlyList<string> Errors) : Result<T>;

    /// <summary>Transforms the value of a success; a failure is passed through unchanged.</summary>
    public Result<TOut> Map<TOut>(Func<T, TOut> map)
    {
        throw new NotImplementedException();
    }

    /// <summary>Chains a step that can itself fail; the step is not called on a failure.</summary>
    public Result<TOut> Bind<TOut>(Func<T, Result<TOut>> next)
    {
        throw new NotImplementedException();
    }

    /// <summary>Collapses the result into a single value.</summary>
    public TOut Match<TOut>(Func<T, TOut> ok, Func<IReadOnlyList<string>, TOut> failure)
    {
        throw new NotImplementedException();
    }
}

/// <summary>Factory and combinator helpers for <see cref="Result{T}"/>.</summary>
public static class Result
{
    public static Result<T> Ok<T>(T value)
    {
        throw new NotImplementedException();
    }

    public static Result<T> Fail<T>(params string[] errors)
    {
        throw new NotImplementedException();
    }

    /// <summary>Succeeds with the value when the predicate holds, otherwise fails with the message.</summary>
    public static Result<T> Ensure<T>(T value, Func<T, bool> predicate, string error)
    {
        throw new NotImplementedException();
    }

    /// <summary>Parses text with any <see cref="IParsable{TSelf}"/> type using the invariant culture.</summary>
    public static Result<T> Parse<T>(string? text, string error)
        where T : IParsable<T>
    {
        throw new NotImplementedException();
    }

    /// <summary>
    /// All values when every result succeeded; otherwise every error of every failure, in order
    /// (no short-circuiting, so a form can show all problems at once).
    /// </summary>
    public static Result<IReadOnlyList<T>> Combine<T>(IEnumerable<Result<T>> results)
    {
        throw new NotImplementedException();
    }
}
