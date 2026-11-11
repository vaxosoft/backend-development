using M12.TaskApi;
using Microsoft.Extensions.Time.Testing;
using NSubstitute;

namespace M12.Exercises;

public class Ex2_ReminderServiceTests
{
    private readonly ITaskStore _store = Substitute.For<ITaskStore>();
    private readonly IReminderNotifier _notifier = Substitute.For<IReminderNotifier>();
    private readonly FakeTimeProvider _clock = new(new DateTimeOffset(2030, 1, 15, 23, 59, 0, TimeSpan.Zero));
    private readonly Ex2_ReminderService _service;

    public Ex2_ReminderServiceTests() => _service = new Ex2_ReminderService(_store, _notifier, _clock);

    private static TaskItem Item(int id, string status, string? due) =>
        new(id, $"task {id}", status, due is null ? null : DateOnly.Parse(due), DateTimeOffset.UnixEpoch);

    private void StoreHolds(params TaskItem[] tasks) =>
        _store.ListAsync(Arg.Any<string?>(), Arg.Any<CancellationToken>()).Returns(tasks);

    [Fact]
    public async Task NotifiesOnlyOverdueTasksThatAreNotDone()
    {
        StoreHolds(
            Item(1, "todo", "2030-01-14"),
            Item(2, "done", "2030-01-01"),
            Item(3, "todo", "2030-01-15"),
            Item(4, "todo", null),
            Item(5, "in_progress", "2029-12-31"));

        var sent = await _service.SendOverdueRemindersAsync();

        Assert.Equal(2, sent);
        await _notifier.Received(1).NotifyAsync(Arg.Is<TaskItem>(t => t.Id == 1), Arg.Any<CancellationToken>());
        await _notifier.Received(1).NotifyAsync(Arg.Is<TaskItem>(t => t.Id == 5), Arg.Any<CancellationToken>());
        await _notifier.Received(2).NotifyAsync(Arg.Any<TaskItem>(), Arg.Any<CancellationToken>());
    }

    [Fact]
    public async Task SendsInDueDateOrder()
    {
        StoreHolds(Item(1, "todo", "2030-01-10"), Item(2, "todo", "2030-01-02"), Item(3, "todo", "2030-01-05"));
        var order = new List<int>();
        await _notifier.NotifyAsync(Arg.Do<TaskItem>(t => order.Add(t.Id)), Arg.Any<CancellationToken>());

        await _service.SendOverdueRemindersAsync();

        Assert.Equal([2, 3, 1], order);
    }

    [Fact]
    public async Task TheClockDecidesWhatIsOverdue()
    {
        StoreHolds(Item(1, "todo", "2030-01-15"));

        Assert.Equal(0, await _service.SendOverdueRemindersAsync());

        _clock.Advance(TimeSpan.FromMinutes(2));

        Assert.Equal(1, await _service.SendOverdueRemindersAsync());
    }

    [Fact]
    public async Task AFailingNotificationDoesNotStopTheOthers()
    {
        StoreHolds(Item(1, "todo", "2030-01-01"), Item(2, "todo", "2030-01-02"), Item(3, "todo", "2030-01-03"));
        _notifier.NotifyAsync(Arg.Is<TaskItem>(t => t.Id == 2), Arg.Any<CancellationToken>())
            .Returns(System.Threading.Tasks.Task.FromException(new InvalidOperationException("smtp down")));

        var failure = await Assert.ThrowsAsync<AggregateException>(() => _service.SendOverdueRemindersAsync());

        Assert.IsType<InvalidOperationException>(Assert.Single(failure.InnerExceptions));
        await _notifier.Received(1).NotifyAsync(Arg.Is<TaskItem>(t => t.Id == 3), Arg.Any<CancellationToken>());
    }

    [Fact]
    public async Task PassesTheCancellationTokenThrough()
    {
        StoreHolds(Item(1, "todo", "2030-01-01"));
        using var cts = new CancellationTokenSource();

        await _service.SendOverdueRemindersAsync(cts.Token);

        await _store.Received(1).ListAsync(Arg.Any<string?>(), cts.Token);
        await _notifier.Received(1).NotifyAsync(Arg.Any<TaskItem>(), cts.Token);
    }
}
