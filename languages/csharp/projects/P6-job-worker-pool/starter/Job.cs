using System.Text.Json;

namespace P6.JobWorkerPool;

public enum JobStatus
{
    Queued,
    Running,
    Succeeded,
    Failed,
    Cancelled,
}

/// <summary>Body of <c>POST /jobs</c>.</summary>
public sealed record JobRequest(string? Type, JsonElement? Payload);

/// <summary>Immutable snapshot returned to clients, so readers never see a half-updated job.</summary>
public sealed record JobView(
    int Id,
    string Type,
    JobStatus Status,
    int Attempts,
    object? Result,
    string? Error,
    DateTimeOffset CreatedAt);

/// <summary>A unit of work. All state changes go through methods that hold the lock and follow the state machine.</summary>
public sealed class Job(int id, string type, JsonElement payload, DateTimeOffset createdAt)
{
    private readonly Lock _gate = new();
    private readonly CancellationTokenSource _cancel = new();
    private JobStatus _status = JobStatus.Queued;
    private int _attempts;
    private object? _result;
    private string? _error;

    public int Id => id;

    public string Type => type;

    public JsonElement Payload => payload;

    /// <summary>Signalled by <see cref="TryCancel"/>.</summary>
    public CancellationToken Token => _cancel.Token;

    public JobView Snapshot()
    {
        lock (_gate)
        {
            return new JobView(id, type, _status, _attempts, _result, _error, createdAt);
        }
    }

    /// <summary>Queued to running. Returns false if the job was cancelled while it waited.</summary>
    public bool TryStart()
    {
        lock (_gate)
        {
            if (_status != JobStatus.Queued)
            {
                return false;
            }

            _status = JobStatus.Running;
            return true;
        }
    }

    public void BeginAttempt()
    {
        lock (_gate)
        {
            _attempts++;
        }
    }

    public void Succeed(object? result)
    {
        lock (_gate)
        {
            if (_status != JobStatus.Running)
            {
                return;
            }

            _status = JobStatus.Succeeded;
            _result = result;
            _error = null;
        }
    }

    public void Fail(string error)
    {
        lock (_gate)
        {
            if (_status != JobStatus.Running)
            {
                return;
            }

            _status = JobStatus.Failed;
            _error = error;
        }
    }

    /// <summary>Queued or running to cancelled. Returns false for a job that already finished.</summary>
    public bool TryCancel()
    {
        lock (_gate)
        {
            if (_status is not (JobStatus.Queued or JobStatus.Running))
            {
                return false;
            }

            _status = JobStatus.Cancelled;
        }

        _cancel.Cancel();
        return true;
    }
}
