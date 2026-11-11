using System.Text.Json;
using System.Text.Json.Serialization;
using P6.JobWorkerPool;

var builder = WebApplication.CreateBuilder(args);

builder.WebHost.UseUrls($"http://{Environment.GetEnvironmentVariable("ADDR") ?? "127.0.0.1:8080"}");

builder.Services.ConfigureHttpJsonOptions(o =>
{
    o.SerializerOptions.PropertyNamingPolicy = JsonNamingPolicy.SnakeCaseLower;
    o.SerializerOptions.Converters.Add(new JsonStringEnumConverter(JsonNamingPolicy.SnakeCaseLower));
});

builder.Services.AddOptions<JobsOptions>()
    .BindConfiguration(JobsOptions.SectionName)
    .ValidateDataAnnotations()
    .ValidateOnStart();

builder.Services.AddSingleton(TimeProvider.System);
builder.Services.AddSingleton<JobStore>();
builder.Services.AddSingleton<JobService>();
builder.Services.AddHostedService<WorkerPool>();

var app = builder.Build();
app.MapJobs();
app.Run();

public partial class Program;
