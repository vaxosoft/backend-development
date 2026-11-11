using System.Net;
using System.Net.Http.Json;
using M12.TaskApi;
using Microsoft.AspNetCore.Hosting;
using Microsoft.AspNetCore.Mvc.Testing;
using Microsoft.AspNetCore.TestHost;
using Microsoft.Extensions.DependencyInjection;
using Microsoft.Extensions.Logging;
using Microsoft.Extensions.Time.Testing;

namespace M12.Exercises;

/// <summary>
/// One behavioural contract, run against every <see cref="ITaskStore"/> implementation.
/// A fake is only trustworthy if it passes the same tests as the real thing.
/// </summary>
public abstract class Ex3_TaskStoreContract
{
    private static readonly DateTimeOffset Created = new(2030, 1, 1, 9, 0, 0, TimeSpan.Zero);

    protected abstract ITaskStore CreateStore();

    [Fact]
    public async Task Add_AssignsSequentialIdsAndDefaults()
    {
        var store = CreateStore();

        var first = await store.AddAsync(new NewTask("one", new DateOnly(2030, 2, 1)), Created);
        var second = await store.AddAsync(new NewTask("two", null), Created.AddHours(1));

        Assert.Equal(new TaskItem(1, "one", "todo", new DateOnly(2030, 2, 1), Created), first);
        Assert.Equal(2, second.Id);
        Assert.Null(second.DueDate);
        Assert.Equal(Created.AddHours(1), second.CreatedAt);
    }

    [Fact]
    public async Task Get_ReturnsTheStoredTaskOrNull()
    {
        var store = CreateStore();
        var added = await store.AddAsync(new NewTask("one", null), Created);

        Assert.Equal(added, await store.GetAsync(added.Id));
        Assert.Null(await store.GetAsync(999));
    }

    [Fact]
    public async Task List_IsOrderedByIdAndFiltersByStatus()
    {
        var store = CreateStore();
        foreach (var title in new[] { "a", "b", "c" })
        {
            await store.AddAsync(new NewTask(title, null), Created);
        }

        await store.SetStatusAsync(2, "done");

        Assert.Equal([1, 2, 3], (await store.ListAsync()).Select(t => t.Id));
        Assert.Equal([2], (await store.ListAsync("done")).Select(t => t.Id));
        Assert.Equal([1, 3], (await store.ListAsync("todo")).Select(t => t.Id));
        Assert.Empty(await store.ListAsync("in_progress"));
    }

    [Fact]
    public async Task SetStatus_ReturnsTheUpdatedTaskOrNull()
    {
        var store = CreateStore();
        await store.AddAsync(new NewTask("one", null), Created);

        var updated = await store.SetStatusAsync(1, "in_progress");

        Assert.Equal("in_progress", updated?.Status);
        Assert.Equal("in_progress", (await store.GetAsync(1))?.Status);
        Assert.Null(await store.SetStatusAsync(42, "done"));
    }

    [Fact]
    public async Task Delete_RemovesOnceAndIdsAreNotReused()
    {
        var store = CreateStore();
        await store.AddAsync(new NewTask("one", null), Created);

        Assert.True(await store.DeleteAsync(1));
        Assert.False(await store.DeleteAsync(1));
        Assert.Null(await store.GetAsync(1));
        Assert.Equal(2, (await store.AddAsync(new NewTask("two", null), Created)).Id);
    }

    [Fact]
    public async Task ConcurrentAdds_GetDistinctIds()
    {
        var store = CreateStore();

        var added = await Task.WhenAll(
            Enumerable.Range(0, 50).Select(i => Task.Run(() => store.AddAsync(new NewTask($"t{i}", null), Created))));

        Assert.Equal(Enumerable.Range(1, 50), added.Select(t => t.Id).Order());
        Assert.Equal(50, (await store.ListAsync()).Count);
    }
}

public class Ex3_InMemoryStoreTests : Ex3_TaskStoreContract
{
    protected override ITaskStore CreateStore() => new Ex3_InMemoryTaskStore();
}

public class Ex3_SqliteStoreTests : Ex3_TaskStoreContract
{
    protected override ITaskStore CreateStore() => new SqliteTaskStore();
}

public class Ex3_ApiWithFakeStoreTests
{
    private static WebApplicationFactory<Program> Factory(FakeTimeProvider clock, ITaskStore store) =>
        new WebApplicationFactory<Program>().WithWebHostBuilder(host =>
        {
            host.ConfigureLogging(logging => logging.ClearProviders());
            host.ConfigureTestServices(services =>
            {
                services.AddSingleton<TimeProvider>(clock);
                services.AddSingleton(store);
            });
        });

    [Fact]
    public async Task PostedTask_GoesThroughTheFakeStoreWithTheFakeClock()
    {
        var clock = new FakeTimeProvider(new DateTimeOffset(2030, 1, 1, 9, 0, 0, TimeSpan.Zero));
        var store = new Ex3_InMemoryTaskStore();
        await using var factory = Factory(clock, store);
        using var client = factory.CreateClient();

        var response = await client.PostAsJsonAsync("/tasks", new { title = "  Write docs  ", due_date = "2030-01-31" });

        Assert.Equal(HttpStatusCode.Created, response.StatusCode);
        Assert.Equal("/tasks/1", response.Headers.Location?.OriginalString);
        Assert.Equal(
            """{"id":1,"title":"Write docs","status":"todo","due_date":"2030-01-31","created_at":"2030-01-01T09:00:00Z"}""",
            await response.Content.ReadAsStringAsync());
        Assert.Single(await store.ListAsync());
    }

    [Fact]
    public async Task ListEndpoint_FiltersByStatusThroughTheStore()
    {
        var store = new Ex3_InMemoryTaskStore();
        await store.AddAsync(new NewTask("a", null), DateTimeOffset.UnixEpoch);
        await store.AddAsync(new NewTask("b", null), DateTimeOffset.UnixEpoch);
        await store.SetStatusAsync(2, "done");
        await using var factory = Factory(new FakeTimeProvider(), store);
        using var client = factory.CreateClient();

        var body = await client.GetFromJsonAsync<System.Text.Json.JsonElement>("/tasks?status=done");

        Assert.Equal(1, body.GetProperty("total").GetInt32());
        Assert.Equal("b", body.GetProperty("items")[0].GetProperty("title").GetString());
    }
}
