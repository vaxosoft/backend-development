using System.Net;
using System.Net.Http.Headers;
using System.Net.Http.Json;
using System.Security.Cryptography;
using System.Text;
using System.Text.Json;
using System.Text.RegularExpressions;

namespace P3.FileService.Tests;

public sealed partial class FileServiceTests : IDisposable
{
    private readonly FilesApi _api = new();

    public void Dispose() => _api.Dispose();

    [GeneratedRegex("^[0-9a-f]{32}$")]
    private static partial Regex IdPattern();

    private static string Sha256Hex(byte[] bytes) => Convert.ToHexStringLower(SHA256.HashData(bytes));

    private static string RawDisposition(HttpResponseMessage response) =>
        response.Content.Headers.NonValidated["Content-Disposition"].ToString();

    private static byte[] Bytes(string text) => Encoding.UTF8.GetBytes(text);

    // --- upload -----------------------------------------------------------

    [Fact]
    public async Task Upload_Returns201WithMetadataAndLocation()
    {
        var response = await _api.CreateClient().PostAsync("/files", FilesApi.Upload(Bytes("hello"), "greeting.txt", "text/plain"));

        Assert.Equal(HttpStatusCode.Created, response.StatusCode);
        var body = await response.Content.ReadFromJsonAsync<JsonElement>();
        var id = body.GetProperty("id").GetString()!;
        Assert.Matches(IdPattern(), id);
        Assert.Equal($"/files/{id}", response.Headers.Location?.OriginalString);
        Assert.Equal("greeting.txt", body.GetProperty("name").GetString());
        Assert.Equal(5, body.GetProperty("size").GetInt64());
        Assert.Equal("text/plain", body.GetProperty("content_type").GetString());
    }

    [Fact]
    public async Task Upload_ReportsTheSha256OfTheContent()
    {
        var body = await _api.UploadOkAsync(Bytes("hello"));

        Assert.Equal(Sha256Hex(Bytes("hello")), body.GetProperty("sha256").GetString());
    }

    [Fact]
    public async Task Upload_AcceptsAnEmptyFile()
    {
        var body = await _api.UploadOkAsync([]);

        Assert.Equal(0, body.GetProperty("size").GetInt64());
        Assert.Equal(Sha256Hex([]), body.GetProperty("sha256").GetString());
    }

    [Fact]
    public async Task Upload_WithoutPartContentType_DefaultsToOctetStream()
    {
        var body = await _api.UploadOkAsync(Bytes("x"), contentType: null);

        Assert.Equal("application/octet-stream", body.GetProperty("content_type").GetString());
    }

    [Fact]
    public async Task Upload_SkipsOtherFormFieldsBeforeTheFile()
    {
        using var form = new MultipartFormDataContent { { new StringContent("a note"), "note" } };
        form.Add(new ByteArrayContent(Bytes("payload")), "file", "p.txt");

        var response = await _api.CreateClient().PostAsync("/files", form);

        Assert.Equal(HttpStatusCode.Created, response.StatusCode);
    }

    [Fact]
    public async Task Upload_UsesAFreshGeneratedIdEachTime()
    {
        var first = await _api.UploadOkAsync(Bytes("same"), "same.txt");
        var second = await _api.UploadOkAsync(Bytes("same"), "same.txt");

        Assert.NotEqual(first.GetProperty("id").GetString(), second.GetProperty("id").GetString());
    }

    [Fact]
    public async Task Upload_WithoutAFilePart_Returns400()
    {
        using var form = new MultipartFormDataContent { { new StringContent("just text"), "note" } };

        var response = await _api.CreateClient().PostAsync("/files", form);

        Assert.Equal(HttpStatusCode.BadRequest, response.StatusCode);
        Assert.Equal("application/problem+json", response.Content.Headers.ContentType?.MediaType);
    }

    [Fact]
    public async Task Upload_ThatIsNotMultipart_Returns400()
    {
        var response = await _api.CreateClient().PostAsync("/files", new StringContent("{}", Encoding.UTF8, "application/json"));

        Assert.Equal(HttpStatusCode.BadRequest, response.StatusCode);
    }

    // --- size limit and streaming -------------------------------------------

    [Theory]
    [InlineData(1024, HttpStatusCode.Created)]
    [InlineData(1025, HttpStatusCode.RequestEntityTooLarge)]
    public async Task Upload_EnforcesTheConfiguredLimit(int size, HttpStatusCode expected)
    {
        using var api = new FilesApi(maxBytes: 1024);

        var response = await api.CreateClient().PostAsync("/files", FilesApi.Upload(new byte[size]));

        Assert.Equal(expected, response.StatusCode);
    }

    [Fact]
    public async Task Upload_TooLarge_Returns413ProblemJson()
    {
        using var api = new FilesApi(maxBytes: 10);

        var response = await api.CreateClient().PostAsync("/files", FilesApi.Upload(new byte[11]));

        Assert.Equal(HttpStatusCode.RequestEntityTooLarge, response.StatusCode);
        Assert.Equal("application/problem+json", response.Content.Headers.ContentType?.MediaType);
    }

    [Fact]
    public async Task Upload_TooLarge_LeavesNothingOnDisk()
    {
        using var api = new FilesApi(maxBytes: 100_000);

        await api.CreateClient().PostAsync("/files", FilesApi.Upload(new byte[300_000]));

        Assert.Empty(Directory.GetFileSystemEntries(api.StorageDir));
    }

    [Fact]
    public async Task Upload_DefaultLimitIs5MiB()
    {
        const int fiveMiB = 5 * 1024 * 1024;
        var client = _api.CreateClient();

        var atLimit = await client.PostAsync("/files", FilesApi.Upload(new byte[fiveMiB]));
        var overLimit = await client.PostAsync("/files", FilesApi.Upload(new byte[fiveMiB + 1]));

        Assert.Equal(HttpStatusCode.Created, atLimit.StatusCode);
        Assert.Equal(HttpStatusCode.RequestEntityTooLarge, overLimit.StatusCode);
    }

    [Fact]
    public async Task Upload_IsStreamed_TheServerStopsReadingOnceTheLimitIsExceeded()
    {
        using var api = new FilesApi(maxBytes: 1024);
        api.UseKestrel(0); // the in-memory test server would swallow the whole body; a real socket applies back-pressure
        api.StartServer();
        var generated = new CountingContent(total: 256L * 1024 * 1024);
        using var form = new MultipartFormDataContent { { generated, "file", "huge.bin" } };

        try
        {
            var response = await api.CreateDefaultClient().PostAsync("/files", form);
            Assert.Equal(HttpStatusCode.RequestEntityTooLarge, response.StatusCode);
        }
        catch (HttpRequestException)
        {
            // Also fine: the server dropped the connection while the client was still uploading.
        }

        Assert.True(generated.Produced < 16L * 1024 * 1024, $"the server pulled {generated.Produced} bytes before refusing");
    }

    // --- file names ---------------------------------------------------------

    [Theory]
    [InlineData("../../etc/passwd", "passwd")]
    [InlineData("..\\..\\boot.ini", "boot.ini")]
    [InlineData("/abs/path/report.pdf", "report.pdf")]
    [InlineData("a/b/c.txt", "c.txt")]
    [InlineData("evil\u0001\u0007.txt", "evil.txt")]
    [InlineData("..", "file")]
    [InlineData("dir/", "file")]
    public async Task Upload_SanitizesTheClientFileName(string clientName, string expected)
    {
        var body = await _api.UploadOkAsync(Bytes("x"), clientName);

        Assert.Equal(expected, body.GetProperty("name").GetString());
    }

    [Fact]
    public async Task Upload_NeverWritesOutsideTheStorageDirectory()
    {
        await _api.UploadOkAsync(Bytes("x"), "../../escaped.txt");
        await _api.UploadOkAsync(Bytes("y"), "..\\..\\escaped2.txt");

        Assert.Equal([_api.StorageDir], Directory.GetFileSystemEntries(_api.Parent));
        var stored = Directory.GetFileSystemEntries(_api.StorageDir);
        Assert.NotEmpty(stored);
        Assert.All(stored, path =>
        {
            Assert.True(File.Exists(path), $"{path} should be a plain file");
            Assert.DoesNotContain("escaped", Path.GetFileName(path));
        });
    }

    // --- download -----------------------------------------------------------

    [Fact]
    public async Task Download_ReturnsTheBytesWithTheExpectedHeaders()
    {
        var content = Enumerable.Range(0, 256).Select(i => (byte)i).ToArray();
        var uploaded = await _api.UploadOkAsync(content, "bytes.bin", "application/x-custom");
        var id = uploaded.GetProperty("id").GetString();

        var response = await _api.CreateClient().GetAsync($"/files/{id}");

        Assert.Equal(HttpStatusCode.OK, response.StatusCode);
        Assert.Equal(content, await response.Content.ReadAsByteArrayAsync());
        Assert.Equal(256, response.Content.Headers.ContentLength);
        Assert.Equal("application/x-custom", response.Content.Headers.ContentType?.MediaType);
        Assert.StartsWith("attachment; filename=\"bytes.bin\"", RawDisposition(response));
        Assert.Equal($"\"{Sha256Hex(content)}\"", response.Headers.ETag?.Tag);
    }

    [Fact]
    public async Task Download_UsesTheSanitizedNameInContentDisposition()
    {
        var uploaded = await _api.UploadOkAsync(Bytes("x"), "../../etc/passwd");

        var response = await _api.CreateClient().GetAsync($"/files/{uploaded.GetProperty("id").GetString()}");

        Assert.StartsWith("attachment; filename=\"passwd\"", RawDisposition(response));
    }

    [Fact]
    public async Task Download_KeepsNonAsciiNamesIntact()
    {
        var uploaded = await _api.UploadOkAsync(Bytes("x"), "r\u00e9sum\u00e9.pdf");

        var response = await _api.CreateClient().GetAsync($"/files/{uploaded.GetProperty("id").GetString()}");

        Assert.Equal("r\u00e9sum\u00e9.pdf", uploaded.GetProperty("name").GetString());
        Assert.Contains("filename*=UTF-8''r%C3%A9sum%C3%A9.pdf", RawDisposition(response));
        Assert.DoesNotContain('\u00e9', RawDisposition(response));
    }

    [Fact]
    public async Task Download_WithMatchingIfNoneMatch_Returns304WithoutBody()
    {
        var uploaded = await _api.UploadOkAsync(Bytes("cache me"));
        var url = $"/files/{uploaded.GetProperty("id").GetString()}";
        using var request = new HttpRequestMessage(HttpMethod.Get, url);
        request.Headers.IfNoneMatch.Add(new EntityTagHeaderValue($"\"{uploaded.GetProperty("sha256").GetString()}\""));

        var response = await _api.CreateClient().SendAsync(request);

        Assert.Equal(HttpStatusCode.NotModified, response.StatusCode);
        Assert.Empty(await response.Content.ReadAsByteArrayAsync());
        Assert.NotNull(response.Headers.ETag);
    }

    [Fact]
    public async Task Download_WithDifferentIfNoneMatch_Returns200()
    {
        var uploaded = await _api.UploadOkAsync(Bytes("cache me"));
        using var request = new HttpRequestMessage(HttpMethod.Get, $"/files/{uploaded.GetProperty("id").GetString()}");
        request.Headers.IfNoneMatch.Add(new EntityTagHeaderValue("\"stale\""));

        var response = await _api.CreateClient().SendAsync(request);

        Assert.Equal(HttpStatusCode.OK, response.StatusCode);
        Assert.Equal("cache me", await response.Content.ReadAsStringAsync());
    }

    [Theory]
    [InlineData("00000000000000000000000000000000")]
    [InlineData("not-an-id")]
    [InlineData("..%2F..%2Fetc%2Fpasswd")]
    [InlineData("%2E%2E")]
    public async Task Download_OfUnknownOrMalformedId_Returns404(string id)
    {
        var response = await _api.CreateClient().GetAsync($"/files/{id}");

        Assert.Equal(HttpStatusCode.NotFound, response.StatusCode);
        Assert.Equal("application/problem+json", response.Content.Headers.ContentType?.MediaType);
    }

    // --- delete -------------------------------------------------------------

    [Fact]
    public async Task Delete_Returns204RemovesTheFileAndItIsGone()
    {
        var uploaded = await _api.UploadOkAsync(Bytes("bye"));
        var url = $"/files/{uploaded.GetProperty("id").GetString()}";
        var client = _api.CreateClient();

        var deleted = await client.DeleteAsync(url);
        var fetched = await client.GetAsync(url);

        Assert.Equal(HttpStatusCode.NoContent, deleted.StatusCode);
        Assert.Equal(HttpStatusCode.NotFound, fetched.StatusCode);
        Assert.Empty(Directory.GetFileSystemEntries(_api.StorageDir));
    }

    [Theory]
    [InlineData("ffffffffffffffffffffffffffffffff")]
    [InlineData("..%2F..%2Fetc%2Fpasswd")]
    public async Task Delete_OfUnknownOrMalformedId_Returns404(string id)
    {
        var response = await _api.CreateClient().DeleteAsync($"/files/{id}");

        Assert.Equal(HttpStatusCode.NotFound, response.StatusCode);
    }
}
