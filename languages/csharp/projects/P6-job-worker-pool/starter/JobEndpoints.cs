namespace P6.JobWorkerPool;

public static class JobEndpoints
{
    public static IEndpointRouteBuilder MapJobs(this IEndpointRouteBuilder app)
    {
        var jobs = app.MapGroup("/jobs");
        jobs.MapPost("/", Submit);
        jobs.MapGet("/", List);
        jobs.MapGet("/{id:int}", Get);
        jobs.MapDelete("/{id:int}", Cancel);
        return app;
    }

    private static IResult Submit(JobRequest request, JobService jobs, HttpResponse response)
    {
        var errors = JobHandlers.Validate(request);
        if (errors.Count > 0)
        {
            return Results.ValidationProblem(errors, statusCode: 422, extensions: Code("validation_failed"));
        }

        var (outcome, job) = jobs.TrySubmit(request.Type!, request.Payload ?? default);
        switch (outcome)
        {
            case SubmitOutcome.Accepted:
                return Results.Accepted($"/jobs/{job!.Id}", new { id = job.Id });
            case SubmitOutcome.QueueFull:
                response.Headers.RetryAfter = "1";
                return Results.Problem("The job queue is full, retry later.", statusCode: 429, extensions: Code("rate_limited"));
            default:
                return Results.Problem("The service is shutting down.", statusCode: 503, extensions: Code("unavailable"));
        }
    }

    private static IResult List(JobStore store)
    {
        var items = store.Newest().Select(j => j.Snapshot()).ToList();
        return Results.Ok(new { items, total = items.Count });
    }

    private static IResult Get(int id, JobStore store) =>
        store.Get(id) is { } job
            ? Results.Ok(job.Snapshot())
            : Results.Problem("No such job.", statusCode: 404, extensions: Code("not_found"));

    private static IResult Cancel(int id, JobService jobs) => jobs.Cancel(id) switch
    {
        CancelOutcome.Cancelled => Results.NoContent(),
        CancelOutcome.NotFound => Results.Problem("No such job.", statusCode: 404, extensions: Code("not_found")),
        _ => Results.Problem("The job already finished.", statusCode: 409, extensions: Code("conflict")),
    };

    // Same status codes as the other tracks, plus a machine-readable "code" member next to the RFC 9457 fields.
    private static Dictionary<string, object?> Code(string code) => new() { ["code"] = code };
}
