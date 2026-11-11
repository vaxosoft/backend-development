using System.Text.Json;
using Microsoft.AspNetCore.Routing;
using P3.FileService;

var builder = WebApplication.CreateBuilder(args);
builder.WebHost.UseUrls($"http://{Environment.GetEnvironmentVariable("ADDR") ?? "127.0.0.1:8080"}");

builder.Services.ConfigureHttpJsonOptions(options =>
    options.SerializerOptions.PropertyNamingPolicy = JsonNamingPolicy.SnakeCaseLower);
builder.Services.AddProblemDetails();
builder.Services.Configure<RouteHandlerOptions>(options => options.ThrowOnBadRequest = false);

builder.Services.AddOptions<FileServiceOptions>().BindConfiguration(FileServiceOptions.SectionName);
builder.Services.AddSingleton<FileStore>();

var app = builder.Build();

app.UseExceptionHandler();
app.UseStatusCodePages();
app.MapFileEndpoints();

app.Run();

// Lets WebApplicationFactory<Program> in the tests find the entry point.
public partial class Program;
