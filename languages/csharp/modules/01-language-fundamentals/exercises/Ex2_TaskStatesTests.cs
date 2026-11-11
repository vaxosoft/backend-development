namespace M01.Exercises;

public class Ex2_TaskStatesTests
{
    [Theory]
    [InlineData("todo", TaskState.Todo)]
    [InlineData("in_progress", TaskState.InProgress)]
    [InlineData("done", TaskState.Done)]
    public void ParsesWireNames(string wire, TaskState expected)
    {
        Assert.True(TaskStates.TryParse(wire, out var state));
        Assert.Equal(expected, state);
    }

    [Theory]
    [InlineData(null)]
    [InlineData("")]
    [InlineData("1")]
    [InlineData("Done")]
    [InlineData("InProgress")]
    [InlineData("in progress")]
    [InlineData(" todo")]
    public void RejectsEverythingElse(string? wire)
    {
        Assert.False(TaskStates.TryParse(wire, out _));
    }

    [Theory]
    [InlineData(TaskState.Todo, "todo")]
    [InlineData(TaskState.InProgress, "in_progress")]
    [InlineData(TaskState.Done, "done")]
    public void ToWireRoundTrips(TaskState state, string wire)
    {
        Assert.Equal(wire, TaskStates.ToWire(state));
        Assert.True(TaskStates.TryParse(TaskStates.ToWire(state), out var back));
        Assert.Equal(state, back);
    }

    [Fact]
    public void ToWireRejectsUndefinedValues()
    {
        Assert.Throws<ArgumentOutOfRangeException>(() => TaskStates.ToWire((TaskState)7));
    }

    [Theory]
    [InlineData(TaskState.Todo, TaskState.Todo, true)]
    [InlineData(TaskState.Todo, TaskState.InProgress, true)]
    [InlineData(TaskState.Todo, TaskState.Done, true)]
    [InlineData(TaskState.InProgress, TaskState.Todo, true)]
    [InlineData(TaskState.InProgress, TaskState.Done, true)]
    [InlineData(TaskState.Done, TaskState.Todo, true)]
    [InlineData(TaskState.Done, TaskState.InProgress, false)]
    [InlineData(TaskState.Done, TaskState.Done, true)]
    public void TransitionRules(TaskState from, TaskState to, bool allowed)
    {
        Assert.Equal(allowed, TaskStates.CanTransition(from, to));
    }
}
