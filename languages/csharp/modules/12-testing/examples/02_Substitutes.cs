using M12.TaskApi;
using NSubstitute;
using NSubstitute.ExceptionExtensions;

namespace M12.Examples;

public static class Substitutes
{
    private sealed class TaskCloser(ITaskStore store)
    {
        public async Task<bool> CompleteAsync(int id)
        {
            var task = await store.GetAsync(id);
            if (task is null || task.Status == "done")
            {
                return false;
            }

            await store.SetStatusAsync(id, "done");
            return true;
        }
    }

    private static TaskItem Item(int id, string status) =>
        new(id, "Write docs", status, null, DateTimeOffset.UnixEpoch);

    public static async Task<int> RunAsync(string[] args)
    {
        var store = Substitute.For<ITaskStore>();
        var closer = new TaskCloser(store);

        // Unconfigured async members return a completed task with a default value (null here).
        Console.WriteLine($"unknown id -> {await closer.CompleteAsync(404)}");
        await store.DidNotReceive().SetStatusAsync(Arg.Any<int>(), Arg.Any<string>(), Arg.Any<CancellationToken>());

        store.GetAsync(1, Arg.Any<CancellationToken>()).Returns(Item(1, "todo"));
        Console.WriteLine($"todo task -> {await closer.CompleteAsync(1)}");
        await store.Received(1).SetStatusAsync(1, "done", Arg.Any<CancellationToken>());
        Console.WriteLine("store received SetStatusAsync(1, \"done\") exactly once");

        store.GetAsync(2, Arg.Any<CancellationToken>()).Returns(Item(2, "done"));
        Console.WriteLine($"already done -> {await closer.CompleteAsync(2)}");
        await store.DidNotReceive().SetStatusAsync(2, Arg.Any<string>(), Arg.Any<CancellationToken>());

        store.GetAsync(3, Arg.Any<CancellationToken>()).ThrowsAsync(new InvalidOperationException("db down"));
        try
        {
            await closer.CompleteAsync(3);
        }
        catch (InvalidOperationException ex)
        {
            Console.WriteLine($"store failure surfaces as: {ex.Message}");
        }

        return 0;
    }
}
