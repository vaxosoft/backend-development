using System.Text.Json;
using P7.TestedService;

var builder = WebApplication.CreateBuilder(args);

var parsed = AppConfig.Parse(key => builder.Configuration[key]);
if (!parsed.IsValid)
{
    Console.Error.WriteLine("Invalid configuration:");
    foreach (var error in parsed.Errors)
    {
        Console.Error.WriteLine($"  - {error}");
    }

    return 78;
}

var config = parsed.Config!;
builder.Logging.SetMinimumLevel(config.LogLevel);
builder.WebHost.UseUrls($"http://{builder.Configuration["ADDR"] ?? $"0.0.0.0:{config.Port}"}");

builder.Services.ConfigureHttpJsonOptions(o => o.SerializerOptions.PropertyNamingPolicy = JsonNamingPolicy.SnakeCaseLower);
builder.Services.AddSingleton(config);
builder.Services.AddSingleton(TimeProvider.System);
builder.Services.AddSingleton<ITaskStore, InMemoryTaskStore>();
builder.Services.AddSingleton<ReadinessState>();
builder.Services.AddSingleton<RequestCounter>();

var app = builder.Build();

var readiness = app.Services.GetRequiredService<ReadinessState>();
app.Lifetime.ApplicationStarted.Register(readiness.MarkReady);
app.Lifetime.ApplicationStopping.Register(readiness.MarkNotReady);

app.UseRequestCounting();
app.MapOperations();
app.MapTasks();
app.Run();
return 0;

public partial class Program;
