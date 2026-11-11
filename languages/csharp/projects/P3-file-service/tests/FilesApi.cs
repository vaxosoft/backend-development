using System.Net;
using System.Net.Http.Headers;
using System.Net.Http.Json;
using System.Text.Json;
using Microsoft.AspNetCore.Hosting;
using Microsoft.AspNetCore.Mvc.Testing;
using Microsoft.Extensions.Configuration;

namespace P3.FileService.Tests;

/// <summary>An app whose storage is a private temp directory, removed on dispose.</summary>
public sealed class FilesApi : WebApplicationFactory<Program>
{
    private readonly string _parent = Path.Combine(Path.GetTempPath(), "p3-" + Guid.NewGuid().ToString("N"));
    private readonly long? _maxBytes;

    public FilesApi(long? maxBytes = null) => _maxBytes = maxBytes;

    /// <summary>The unique directory that contains <see cref="StorageDir"/> (to spot escapes).</summary>
    public string Parent => _parent;

    public string StorageDir => Path.Combine(_parent, "storage");

    protected override void ConfigureWebHost(IWebHostBuilder builder) =>
        builder.ConfigureAppConfiguration((_, config) =>
        {
            var settings = new Dictionary<string, string?> { ["FileService:StorageDir"] = StorageDir };
            if (_maxBytes is { } max)
            {
                settings["FileService:MaxUploadBytes"] = max.ToString();
            }

            config.AddInMemoryCollection(settings);
        });

    protected override void Dispose(bool disposing)
    {
        base.Dispose(disposing);
        if (disposing && Directory.Exists(_parent))
        {
            Directory.Delete(_parent, recursive: true);
        }
    }

    public static MultipartFormDataContent Upload(byte[] bytes, string fileName = "a.txt", string? contentType = "text/plain")
    {
        var file = new ByteArrayContent(bytes);
        if (contentType is not null)
        {
            file.Headers.ContentType = MediaTypeHeaderValue.Parse(contentType);
        }

        return new MultipartFormDataContent { { file, "file", fileName } };
    }

    public async Task<JsonElement> UploadOkAsync(byte[] bytes, string fileName = "a.txt", string? contentType = "text/plain")
    {
        var response = await CreateClient().PostAsync("/files", Upload(bytes, fileName, contentType));
        Assert.Equal(HttpStatusCode.Created, response.StatusCode);
        return (await response.Content.ReadFromJsonAsync<JsonElement>()).Clone();
    }
}

/// <summary>Produces <c>total</c> bytes in 64 KiB chunks and counts how many were actually pulled.</summary>
public sealed class CountingContent(long total) : HttpContent
{
    private long _produced;

    public long Produced => Interlocked.Read(ref _produced);

    protected override async Task SerializeToStreamAsync(Stream stream, TransportContext? context)
    {
        var chunk = new byte[64 * 1024];
        for (long sent = 0; sent < total; sent += chunk.Length)
        {
            await stream.WriteAsync(chunk);
            Interlocked.Add(ref _produced, chunk.Length);
        }
    }

    protected override bool TryComputeLength(out long length)
    {
        length = total;
        return true;
    }
}
