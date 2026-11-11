using BenchmarkDotNet.Attributes;
using BenchmarkDotNet.Configs;
using BenchmarkDotNet.Jobs;
using BenchmarkDotNet.Running;
using BenchmarkDotNet.Toolchains.InProcess.NoEmit;

namespace M12.Examples;

public class TagJoinBenchmarks
{
    private readonly string[] _tags = Enumerable.Range(0, 200).Select(i => $"tag-{i}").ToArray();

    [Benchmark(Baseline = true)]
    public string Concatenate()
    {
        var result = "";
        foreach (var tag in _tags)
        {
            result += tag + ",";
        }

        return result;
    }

    [Benchmark]
    public string Join() => string.Join(',', _tags);
}

/// <summary>Swallows BenchmarkDotNet's progress output so the example prints only stable lines.</summary>
internal sealed class QuietLogger : BenchmarkDotNet.Loggers.ILogger
{
    public string Id => nameof(QuietLogger);

    public int Priority => 0;

    public void Write(BenchmarkDotNet.Loggers.LogKind logKind, string text)
    {
    }

    public void WriteLine()
    {
    }

    public void WriteLine(BenchmarkDotNet.Loggers.LogKind logKind, string text)
    {
    }

    public void Flush()
    {
    }
}

public static class Benchmarks
{
    public static Task<int> RunAsync(string[] args)
    {
        var artifacts = Path.Combine(Path.GetTempPath(), $"m12-bdn-{Environment.ProcessId}");
        var config = ManualConfig.CreateEmpty()
            .AddJob(Job.ShortRun.WithToolchain(InProcessNoEmitToolchain.Instance))
            .AddLogger(new QuietLogger())
            .WithArtifactsPath(artifacts);

        var summary = BenchmarkRunner.Run<TagJoinBenchmarks>(config);
        var fastest = summary.Reports.OrderBy(r => r.ResultStatistics!.Mean).First();
        Console.WriteLine($"benchmarks run: {summary.Reports.Length}");
        Console.WriteLine($"fastest: {fastest.BenchmarkCase.Descriptor.WorkloadMethod.Name}");

        Directory.Delete(artifacts, recursive: true);
        return Task.FromResult(summary.HasCriticalValidationErrors ? 1 : 0);
    }
}
