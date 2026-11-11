namespace M02.Examples;

static class ExampleHost
{
    /// <summary>Builder that listens on the ADDR environment variable (host:port, default 127.0.0.1:8080).</summary>
    public static WebApplicationBuilder CreateBuilder(string[] args)
    {
        var builder = WebApplication.CreateBuilder(args);
        builder.WebHost.UseUrls($"http://{Environment.GetEnvironmentVariable("ADDR") ?? "127.0.0.1:8080"}");
        builder.Logging.AddFilter("Microsoft.AspNetCore", LogLevel.Warning);
        return builder;
    }
}
