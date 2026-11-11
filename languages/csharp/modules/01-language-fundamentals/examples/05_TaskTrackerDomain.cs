namespace M01.Examples;

/// <summary>The Task Tracker domain with records, interfaces, composition and LINQ.</summary>
static class TaskTrackerDomain
{
    enum TaskState
    {
        Todo,
        InProgress,
        Done,
    }

    record TaskItem
    {
        public required int Id { get; init; }

        public required string Title { get; init; }

        public TaskState Status { get; init; } = TaskState.Todo;

        public DateOnly? DueDate { get; init; }

        public IReadOnlyList<string> Tags { get; init; } = [];
    }

    interface ITaskStore
    {
        TaskItem Add(string title, DateOnly? due = null, params string[] tags);

        TaskItem? Find(int id);

        IReadOnlyList<TaskItem> All();

        TaskItem? Update(int id, Func<TaskItem, TaskItem> change);
    }

    sealed class InMemoryTaskStore : ITaskStore
    {
        readonly Dictionary<int, TaskItem> _items = [];
        int _lastId;

        public TaskItem Add(string title, DateOnly? due = null, params string[] tags)
        {
            var item = new TaskItem { Id = ++_lastId, Title = title, DueDate = due, Tags = tags };
            _items[item.Id] = item;
            return item;
        }

        public TaskItem? Find(int id) => _items.GetValueOrDefault(id);

        public IReadOnlyList<TaskItem> All() => [.. _items.Values.OrderBy(t => t.Id)];

        public TaskItem? Update(int id, Func<TaskItem, TaskItem> change) =>
            _items.TryGetValue(id, out var current) ? _items[id] = change(current) with { Id = id } : null;
    }

    // Composition: wrap any ITaskStore instead of inheriting from a concrete class.
    sealed class AuditedStore(ITaskStore inner, List<string> audit) : ITaskStore
    {
        public TaskItem Add(string title, DateOnly? due = null, params string[] tags)
        {
            audit.Add($"add '{title}'");
            return inner.Add(title, due, tags);
        }

        public TaskItem? Find(int id) => inner.Find(id);

        public IReadOnlyList<TaskItem> All() => inner.All();

        public TaskItem? Update(int id, Func<TaskItem, TaskItem> change)
        {
            audit.Add($"update #{id}");
            return inner.Update(id, change);
        }
    }

    static string Wire(TaskState s) => s switch
    {
        TaskState.Todo => "todo",
        TaskState.InProgress => "in_progress",
        TaskState.Done => "done",
        _ => throw new ArgumentOutOfRangeException(nameof(s)),
    };

    public static Task<int> RunAsync(string[] args)
    {
        var audit = new List<string>();
        ITaskStore store = new AuditedStore(new InMemoryTaskStore(), audit);

        store.Add("Write docs", new DateOnly(2030, 1, 31), "docs", "p1");
        store.Add("Fix login bug", tags: "bug");
        store.Add("Release", new DateOnly(2030, 2, 1));
        store.Update(2, t => t with { Status = TaskState.InProgress });
        store.Update(1, t => t with { Status = TaskState.Done });

        foreach (var t in store.All())
        {
            var due = t.DueDate?.ToString("yyyy-MM-dd") ?? "no due date";
            Console.WriteLine($"#{t.Id} {t.Title,-14} {Wire(t.Status),-11} {due} [{string.Join(",", t.Tags)}]");
        }

        var open = store.All()
            .Where(t => t.Status != TaskState.Done)
            .OrderBy(t => t.DueDate ?? DateOnly.MaxValue)
            .Select(t => t.Title);
        Console.WriteLine($"open by due date: {string.Join(" > ", open)}");

        foreach (var g in store.All().GroupBy(t => t.Status).OrderBy(g => g.Key))
        {
            Console.WriteLine($"{Wire(g.Key)}: {g.Count()}");
        }

        Console.WriteLine($"missing id 99: {(store.Find(99) is null ? "null" : "found")}");
        Console.WriteLine($"audit: {string.Join("; ", audit)}");
        return Task.FromResult(0);
    }
}
