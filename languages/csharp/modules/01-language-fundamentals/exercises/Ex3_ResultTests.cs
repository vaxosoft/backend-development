namespace M01.Exercises;

public class Ex3_ResultTests
{
    static IReadOnlyList<string> ErrorsOf<T>(Result<T> result) =>
        Assert.IsType<Result<T>.Failure>(result).Errors;

    static T ValueOf<T>(Result<T> result) => Assert.IsType<Result<T>.Ok>(result).Value;

    [Fact]
    public void MapTransformsSuccessAndKeepsFailure()
    {
        Assert.Equal(6, ValueOf(Result.Ok(3).Map(x => x * 2)));
        Assert.Equal(["bad"], ErrorsOf(Result.Fail<int>("bad").Map(x => x * 2)));
    }

    [Fact]
    public void BindChainsAndShortCircuits()
    {
        var calls = 0;
        Result<string> Next(int n)
        {
            calls++;
            return Result.Ok($"n={n}");
        }

        Assert.Equal("n=1", ValueOf(Result.Ok(1).Bind(Next)));
        Assert.Equal(["no"], ErrorsOf(Result.Fail<int>("no").Bind(Next)));
        Assert.Equal(1, calls);
    }

    [Fact]
    public void MatchPicksTheRightBranch()
    {
        Assert.Equal("ok 5", Result.Ok(5).Match(v => $"ok {v}", e => "failed"));
        Assert.Equal("failed a,b", Result.Fail<int>("a", "b").Match(v => "ok", e => $"failed {string.Join(",", e)}"));
    }

    [Fact]
    public void EnsureAppliesThePredicate()
    {
        Assert.Equal("abc", ValueOf(Result.Ensure("abc", s => s.Length > 0, "empty")));
        Assert.Equal(["empty"], ErrorsOf(Result.Ensure("", s => s.Length > 0, "empty")));
    }

    [Fact]
    public void ParseWorksForAnyIParsableType()
    {
        Assert.Equal(42, ValueOf(Result.Parse<int>("42", "not an int")));
        Assert.Equal(new DateOnly(2030, 1, 31), ValueOf(Result.Parse<DateOnly>("2030-01-31", "bad date")));
        Assert.Equal(["bad date"], ErrorsOf(Result.Parse<DateOnly>("31/01/2030", "bad date")));
        Assert.Equal(["not an int"], ErrorsOf(Result.Parse<int>(null, "not an int")));
    }

    [Fact]
    public void ParseUsesInvariantDecimalPoint()
    {
        Assert.Equal(1.5, ValueOf(Result.Parse<double>("1.5", "bad")));
        Assert.Equal(["bad"], ErrorsOf(Result.Parse<double>("1.5.2", "bad")));
    }

    [Fact]
    public void CombineCollectsValuesInOrder()
    {
        var combined = Result.Combine([Result.Ok(1), Result.Ok(2), Result.Ok(3)]);
        Assert.Equal([1, 2, 3], ValueOf(combined));
    }

    [Fact]
    public void CombineAccumulatesEveryError()
    {
        var combined = Result.Combine([Result.Fail<int>("a"), Result.Ok(2), Result.Fail<int>("b", "c")]);
        Assert.Equal(["a", "b", "c"], ErrorsOf(combined));
    }

    [Fact]
    public void CombineOfNothingSucceedsWithEmptyList()
    {
        Assert.Empty(ValueOf(Result.Combine(Array.Empty<Result<int>>())));
    }
}
