using System.Diagnostics;
using System.Diagnostics.Metrics;

namespace M10.Examples;

/// <summary>Example 4: counters and histograms with System.Diagnostics.Metrics, and spans with Activity.</summary>
public static class MetricsAndActivities
{
    public static Task<int> RunAsync(string[] args)
    {
        using var meter = new Meter("TaskTracker");
        var created = meter.CreateCounter<long>("tasks_created", unit: "{task}", description: "Tasks created");
        var duration = meter.CreateHistogram<double>("request_duration", unit: "ms");
        var queued = 4;
        meter.CreateObservableGauge("queue_length", () => queued, unit: "{job}");

        // dotnet-counters / OpenTelemetry attach a listener like this one from the outside.
        using var listener = new MeterListener();
        listener.InstrumentPublished = (instrument, l) =>
        {
            if (instrument.Meter.Name == "TaskTracker")
            {
                l.EnableMeasurementEvents(instrument);
            }
        };
        listener.SetMeasurementEventCallback<long>((instrument, value, tags, _) => Console.WriteLine($"  {instrument.Name} += {value} {Describe(tags)}".TrimEnd()));
        listener.SetMeasurementEventCallback<double>((instrument, value, tags, _) => Console.WriteLine($"  {instrument.Name} recorded {value:0.#} {instrument.Unit} {Describe(tags)}".TrimEnd()));
        listener.SetMeasurementEventCallback<int>((instrument, value, tags, _) => Console.WriteLine($"  {instrument.Name} = {value} {Describe(tags)}".TrimEnd()));
        listener.Start();

        Console.WriteLine("measurements:");
        created.Add(1, new KeyValuePair<string, object?>("status", "todo"));
        created.Add(2, new KeyValuePair<string, object?>("status", "done"));
        duration.Record(12.5, new KeyValuePair<string, object?>("route", "/tasks"));
        listener.RecordObservableInstruments();

        Console.WriteLine("spans:");
        using var source = new ActivitySource("TaskTracker");
        using var activityListener = new ActivityListener
        {
            ShouldListenTo = s => s.Name == "TaskTracker",
            Sample = (ref ActivityCreationOptions<ActivityContext> _) => ActivitySamplingResult.AllDataAndRecorded,
            ActivityStopped = a => Console.WriteLine($"  stopped '{a.OperationName}' parent={(a.Parent?.OperationName ?? "none")} tags=[{string.Join(", ", a.TagObjects.Select(t => $"{t.Key}={t.Value}"))}]"),
        };
        ActivitySource.AddActivityListener(activityListener);

        using (var request = source.StartActivity("POST /tasks"))
        {
            request?.SetTag("http.route", "/tasks");
            using var save = source.StartActivity("SaveTask");
            save?.SetTag("task.id", 5);
        }

        // Quick timing without any listener: Stopwatch.GetElapsedTime is allocation free.
        var start = Stopwatch.GetTimestamp();
        var elapsed = Stopwatch.GetElapsedTime(start);
        Console.WriteLine($"stopwatch elapsed is non-negative: {elapsed >= TimeSpan.Zero}");
        return Task.FromResult(0);
    }

    private static string Describe(ReadOnlySpan<KeyValuePair<string, object?>> tags)
    {
        var parts = new List<string>();
        foreach (var tag in tags)
        {
            parts.Add($"{tag.Key}={tag.Value}");
        }

        return parts.Count == 0 ? "" : $"[{string.Join(", ", parts)}]";
    }
}
