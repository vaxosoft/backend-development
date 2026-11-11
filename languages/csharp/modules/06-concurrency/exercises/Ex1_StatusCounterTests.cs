namespace M06.Exercises;

public class Ex1_StatusCounterTests
{
    [Fact]
    public void Increment_returns_the_running_count()
    {
        var counter = new Ex1_StatusCounter();

        Assert.Equal(1, counter.Increment("todo"));
        Assert.Equal(2, counter.Increment("todo"));
        Assert.Equal(1, counter.Increment("done"));
        Assert.Equal(0, counter.Get("in_progress"));
        Assert.Equal(3, counter.Total);
    }

    [Fact]
    public void Concurrent_increments_lose_nothing()
    {
        var counter = new Ex1_StatusCounter();
        string[] statuses = ["todo", "in_progress", "done"];

        Parallel.For(0, 8, new ParallelOptions { MaxDegreeOfParallelism = 8 }, worker =>
        {
            for (var i = 0; i < 20_000; i++)
            {
                counter.Increment(statuses[(worker + i) % 3]);
            }
        });

        Assert.Equal(160_000, counter.Total);
        Assert.Equal(160_000, counter.Snapshot().Values.Sum());
    }

    [Fact]
    public void Snapshot_is_a_copy()
    {
        var counter = new Ex1_StatusCounter();
        counter.Increment("todo");
        var snapshot = counter.Snapshot();

        counter.Increment("todo");

        Assert.Equal(1, snapshot["todo"]);
        Assert.Equal(2, counter.Get("todo"));
    }

    [Fact]
    public void Limit_is_enforced_exactly_under_contention()
    {
        var counter = new Ex1_StatusCounter();
        var accepted = 0;

        Parallel.For(0, 2_000, new ParallelOptions { MaxDegreeOfParallelism = 16 }, _ =>
        {
            if (counter.TryIncrementBelow("in_progress", 100))
            {
                Interlocked.Increment(ref accepted);
            }
        });

        Assert.Equal(100, accepted);
        Assert.Equal(100, counter.Get("in_progress"));
    }

    [Fact]
    public void Limit_applies_per_status()
    {
        var counter = new Ex1_StatusCounter();

        Assert.True(counter.TryIncrementBelow("a", 1));
        Assert.False(counter.TryIncrementBelow("a", 1));
        Assert.True(counter.TryIncrementBelow("b", 1));
        Assert.False(counter.TryIncrementBelow("c", 0));
    }
}
