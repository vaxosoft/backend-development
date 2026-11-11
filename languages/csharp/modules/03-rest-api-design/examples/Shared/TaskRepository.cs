namespace M03.Examples;

/// <summary>Deterministic clock: every reading is one minute after the previous one.</summary>
sealed class StepClock : TimeProvider
{
    readonly Lock _gate = new();
    DateTimeOffset _now = new(2030, 1, 1, 9, 0, 0, TimeSpan.Zero);

    public override DateTimeOffset GetUtcNow()
    {
        lock (_gate)
        {
            var current = _now;
            _now = _now.AddMinutes(1);
            return current;
        }
    }
}

/// <summary>Thread-safe in-memory storage. The id and creation time are assigned here, never by clients.</summary>
public sealed class TaskRepository(TimeProvider clock)
{
    readonly Lock _gate = new();
    readonly SortedDictionary<int, TaskItem> _items = [];
    int _lastId;

    public TaskItem Add(TaskInput input)
    {
        lock (_gate)
        {
            var item = new TaskItem(++_lastId, input.Title!.Trim(), input.Status, input.DueDate, input.Tags, input.OwnerId,
                clock.GetUtcNow().UtcDateTime);
            _items[item.Id] = item;
            return item;
        }
    }

    public TaskItem? Find(int id)
    {
        lock (_gate)
        {
            return _items.GetValueOrDefault(id);
        }
    }

    public TaskItem? Replace(int id, TaskInput input)
    {
        lock (_gate)
        {
            if (!_items.TryGetValue(id, out var current))
            {
                return null;
            }

            return _items[id] = current with
            {
                Title = input.Title!.Trim(),
                Status = input.Status,
                DueDate = input.DueDate,
                Tags = input.Tags,
                OwnerId = input.OwnerId,
            };
        }
    }

    public bool Remove(int id)
    {
        lock (_gate)
        {
            return _items.Remove(id);
        }
    }

    public IReadOnlyList<TaskItem> All()
    {
        lock (_gate)
        {
            return [.. _items.Values];
        }
    }
}
