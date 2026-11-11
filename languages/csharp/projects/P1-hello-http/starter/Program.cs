using P1.HelloHttp;

var addr = Environment.GetEnvironmentVariable("ADDR") ?? "127.0.0.1:8080";
await HelloApp.RunAsync(addr, CancellationToken.None, onListening: url => Console.WriteLine($"listening on {url}"));

// Lets WebApplicationFactory<Program> in the tests find the entry point.
public partial class Program;
