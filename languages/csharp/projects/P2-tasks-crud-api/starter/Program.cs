using System.Text.Json;
using System.Text.Json.Serialization;
using Microsoft.AspNetCore.Http.Json;
using Microsoft.AspNetCore.Routing;
using Microsoft.Extensions.DependencyInjection.Extensions;
using P2.TasksCrudApi;

var builder = WebApplication.CreateBuilder(args);
builder.WebHost.UseUrls($"http://{Environment.GetEnvironmentVariable("ADDR") ?? "127.0.0.1:8080"}");

builder.Services.ConfigureHttpJsonOptions(options =>
{
    options.SerializerOptions.PropertyNamingPolicy = JsonNamingPolicy.SnakeCaseLower;
    options.SerializerOptions.Converters.Add(new UtcTimestampConverter());
});
builder.Services.AddProblemDetails();
builder.Services.AddOpenApi();

// Binding failures (malformed JSON, ...) become plain 400 responses instead of exceptions in every environment.
builder.Services.Configure<RouteHandlerOptions>(options => options.ThrowOnBadRequest = false);

builder.Services.TryAddSingleton(TimeProvider.System);
builder.Services.AddSingleton<ITaskRepository>(sp => new SqliteTaskRepository(
    sp.GetRequiredService<IConfiguration>().GetConnectionString("Tasks") ?? "Data Source=tasks.db",
    sp.GetRequiredService<TimeProvider>()));

var app = builder.Build();

app.Services.GetRequiredService<ITaskRepository>(); // create the schema at startup, not on the first request

app.UseExceptionHandler();
app.UseStatusCodePages();
app.MapOpenApi("/openapi.json");
app.MapTaskEndpoints();

app.Run();

// Lets WebApplicationFactory<Program> in the tests find the entry point.
public partial class Program;
