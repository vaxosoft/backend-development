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
    public async Task<int> SendOverdueRemindersAsync(CancellationToken ct = default)
    {
        var today = DateOnly.FromDateTime(time.GetUtcNow().UtcDateTime);
        var overdue = (await store.ListAsync(null, ct))
            .Where(t => t.Status != "done" && t.DueDate < today)
            .OrderBy(t => t.DueDate)
            .ToList();

        var failures = new List<Exception>();
        var sent = 0;
        foreach (var task in overdue)
        {
            try
            {
                await notifier.NotifyAsync(task, ct);
                sent++;
            }
            catch (Exception ex) when (ex is not OperationCanceledException)
            {
                failures.Add(ex);
            }
        }

        return failures.Count == 0 ? sent : throw new AggregateException(failures);
    }
}
