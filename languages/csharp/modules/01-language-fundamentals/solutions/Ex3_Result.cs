using System.Globalization;

namespace M01.Exercises;

/// <summary>Either a value or a list of error messages.</summary>
public abstract record Result<T>
{
    /// <summary>Success.</summary>
    public sealed record Ok(T Value) : Result<T>;

    /// <summary>Failure with at least one message.</summary>
    public sealed record Failure(IReadOnlyList<string> Errors) : Result<T>;

    /// <summary>Transforms the value of a success; a failure is passed through unchanged.</summary>
    public Result<TOut> Map<TOut>(Func<T, TOut> map) => this switch
    {
        Ok(var value) => new Result<TOut>.Ok(map(value)),
        Failure(var errors) => new Result<TOut>.Failure(errors),
        _ => throw new InvalidOperationException(),
    };

    /// <summary>Chains a step that can itself fail; the step is not called on a failure.</summary>
    public Result<TOut> Bind<TOut>(Func<T, Result<TOut>> next) => this switch
    {
        Ok(var value) => next(value),
        Failure(var errors) => new Result<TOut>.Failure(errors),
        _ => throw new InvalidOperationException(),
    };

    /// <summary>Collapses the result into a single value.</summary>
    public TOut Match<TOut>(Func<T, TOut> ok, Func<IReadOnlyList<string>, TOut> failure) => this switch
    {
        Ok(var value) => ok(value),
        Failure(var errors) => failure(errors),
        _ => throw new InvalidOperationException(),
    };
}

/// <summary>Factory and combinator helpers for <see cref="Result{T}"/>.</summary>
public static class Result
{
    public static Result<T> Ok<T>(T value) => new Result<T>.Ok(value);

    public static Result<T> Fail<T>(params string[] errors) => new Result<T>.Failure(errors);

    /// <summary>Succeeds with the value when the predicate holds, otherwise fails with the message.</summary>
    public static Result<T> Ensure<T>(T value, Func<T, bool> predicate, string error) =>
        predicate(value) ? Ok(value) : Fail<T>(error);

    /// <summary>Parses text with any <see cref="IParsable{TSelf}"/> type using the invariant culture.</summary>
    public static Result<T> Parse<T>(string? text, string error)
        where T : IParsable<T> =>
        T.TryParse(text, CultureInfo.InvariantCulture, out var value) ? Ok(value) : Fail<T>(error);

    /// <summary>
    /// All values when every result succeeded; otherwise every error of every failure, in order
    /// (no short-circuiting, so a form can show all problems at once).
    /// </summary>
    public static Result<IReadOnlyList<T>> Combine<T>(IEnumerable<Result<T>> results)
    {
        var values = new List<T>();
        var errors = new List<string>();
        foreach (var result in results)
        {
            switch (result)
            {
                case Result<T>.Ok(var value):
                    values.Add(value);
                    break;
                case Result<T>.Failure(var failed):
                    errors.AddRange(failed);
                    break;
            }
        }

        return errors.Count == 0 ? Ok<IReadOnlyList<T>>(values) : Fail<IReadOnlyList<T>>([.. errors]);
    }
}
