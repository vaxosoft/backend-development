using System.Collections.Concurrent;

namespace P6.JobWorkerPool;

/// <summary>Thread-safe in-memory registry of every job ever accepted.</summary>
public sealed class JobStore
{
    private readonly ConcurrentDictionary<int, Job> _jobs = new();
    private int _lastId;

    public int NextId() => Interlocked.Increment(ref _lastId);

    public void Add(Job job) => _jobs[job.Id] = job;

    public Job? Get(int id) => _jobs.GetValueOrDefault(id);

    public IReadOnlyList<Job> Newest() => [.. _jobs.Values.OrderByDescending(j => j.Id)];
}
