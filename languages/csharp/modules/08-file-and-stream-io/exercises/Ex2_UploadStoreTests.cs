using System.Security.Cryptography;
using System.Text;

namespace M08.Exercises;

public class Ex2_UploadStoreTests
{
    [Fact]
    public async Task Saves_the_content_and_reports_size_and_sha256()
    {
        using var dir = new TempDir();
        var store = new UploadStore(Path.Combine(dir.Path, "uploads"), maxBytes: 1024);
        var bytes = Encoding.UTF8.GetBytes("hello");

        var stored = await store.SaveAsync(new MemoryStream(bytes));

        Assert.Equal(5, stored.Size);
        Assert.Equal("2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824", stored.Sha256);
        Assert.Matches("^[0-9a-f]{32}$", stored.Id);
        using var reader = new StreamReader(store.Open(stored.Id)!);
        Assert.Equal("hello", await reader.ReadToEndAsync());
    }

    [Fact]
    public async Task Large_inputs_are_hashed_correctly_across_chunk_boundaries()
    {
        using var dir = new TempDir();
        var store = new UploadStore(dir.Path, maxBytes: 10_000_000);
        var bytes = new byte[300_001];
        new Random(42).NextBytes(bytes);

        var stored = await store.SaveAsync(new MemoryStream(bytes));

        Assert.Equal(bytes.Length, stored.Size);
        Assert.Equal(Convert.ToHexStringLower(SHA256.HashData(bytes)), stored.Sha256);
    }

    [Fact]
    public async Task Exactly_the_limit_is_accepted_and_the_empty_file_too()
    {
        using var dir = new TempDir();
        var store = new UploadStore(dir.Path, maxBytes: 10);

        Assert.Equal(10, (await store.SaveAsync(new MemoryStream(new byte[10]))).Size);
        var empty = await store.SaveAsync(new MemoryStream());
        Assert.Equal(0, empty.Size);
        Assert.Equal("e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", empty.Sha256);
    }

    [Fact]
    public async Task An_oversized_upload_is_rejected_without_reading_it_all_and_leaves_nothing_behind()
    {
        using var dir = new TempDir();
        var store = new UploadStore(dir.Path, maxBytes: 1000);
        var endless = new EndlessStream();

        await Assert.ThrowsAsync<FileTooLargeException>(() => store.SaveAsync(endless));

        Assert.True(endless.BytesRead < 1000 + 128 * 1024, $"read {endless.BytesRead} bytes from an endless stream");
        Assert.Empty(Directory.GetFileSystemEntries(dir.Path));
    }

    [Fact]
    public async Task A_cancelled_upload_leaves_no_partial_file()
    {
        using var dir = new TempDir();
        var store = new UploadStore(dir.Path, maxBytes: 1000);
        using var cts = new CancellationTokenSource();
        await cts.CancelAsync();

        await Assert.ThrowsAnyAsync<OperationCanceledException>(() => store.SaveAsync(new EndlessStream(), cts.Token));

        Assert.Empty(Directory.GetFileSystemEntries(dir.Path));
    }

    [Theory]
    [InlineData("../x")]
    [InlineData("..")]
    [InlineData("/etc/passwd")]
    [InlineData("not-a-valid-id")]
    [InlineData("")]
    public void Ids_that_are_not_generated_ids_are_unknown(string id)
    {
        using var dir = new TempDir();
        File.WriteAllText(Path.Combine(Path.GetDirectoryName(dir.Path)!, "x"), "outside");
        var store = new UploadStore(dir.Path, 1000);

        try
        {
            Assert.Null(store.Open(id));
        }
        finally
        {
            File.Delete(Path.Combine(Path.GetDirectoryName(dir.Path)!, "x"));
        }
    }

    [Fact]
    public void An_unknown_but_well_formed_id_is_null_too()
    {
        using var dir = new TempDir();

        Assert.Null(new UploadStore(dir.Path, 1000).Open(new string('a', 32)));
    }
}
