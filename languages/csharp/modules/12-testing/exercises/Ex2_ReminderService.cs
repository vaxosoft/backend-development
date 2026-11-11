#pragma warning disable CS9113 // starter: your implementation will read these parameters

using M12.TaskApi;

namespace M12.Exercises;

public interface IReminderNotifier
{
    Task NotifyAsync(TaskItem task, CancellationToken ct = default);
}

/// <summary>
/// Sends one reminder per overdue task. A task is overdue when it is not "done" and its due date is
/// before today's UTC date (taken from <paramref name="time"/>). Reminders go out in due-date order.
/// </summary>
public sealed class Ex2_ReminderService(ITaskStore store, IReminderNotifier notifier, TimeProvider time)
{
    /// <summary>
    /// Returns how many reminders were sent. If some notifications fail, still attempt all of them,
    /// then throw an <see cref="AggregateException"/> holding every failure.
    /// </summary>
    public Task<int> SendOverdueRemindersAsync(CancellationToken ct = default) =>
        throw new NotImplementedException();
}
