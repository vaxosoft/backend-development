using System.Net;
using Microsoft.Extensions.DependencyInjection;
using static M07.Exercises.ScriptedHandler;

namespace M07.Exercises;

public class Ex3_ResilientClientFactoryTests
{
    private static InventoryClient Create(ScriptedHandler upstream, TimeSpan? attemptTimeout = null) =>
        InventoryClientFactory
            .Build(new Uri("http://inventory.test/v1/"), "key-123", () => upstream, attemptTimeout ?? TimeSpan.FromSeconds(5))
            .GetRequiredService<InventoryClient>();

    [Fact]
    public async Task The_client_is_configured_with_a_base_address_and_api_key()
    {
        var upstream = new ScriptedHandler(Status(HttpStatusCode.OK, "widget"));

        Assert.Equal("widget", await Create(upstream).GetItemAsync(7));

        var seen = Assert.Single(upstream.Seen);
        Assert.Equal("http://inventory.test/v1/items/7", seen.Uri!.ToString());
        Assert.Equal(["key-123"], seen.Headers.GetValues("X-Api-Key"));
    }

    [Fact]
    public async Task Transient_failures_are_retried_until_success()
    {
        var upstream = new ScriptedHandler(Status(HttpStatusCode.ServiceUnavailable), Throw(new HttpRequestException("reset")), Status(HttpStatusCode.OK, "widget"));

        Assert.Equal("widget", await Create(upstream).GetItemAsync(1));
        Assert.Equal(3, upstream.Calls);
    }

    [Fact]
    public async Task Retries_stop_after_two_extra_attempts()
    {
        var upstream = new ScriptedHandler(Status(HttpStatusCode.InternalServerError));

        var ex = await Assert.ThrowsAsync<HttpRequestException>(() => Create(upstream).GetItemAsync(1));

        Assert.Equal(HttpStatusCode.InternalServerError, ex.StatusCode);
        Assert.Equal(3, upstream.Calls);
    }

    [Fact]
    public async Task Client_errors_are_not_retried()
    {
        var upstream = new ScriptedHandler(Status(HttpStatusCode.NotFound));

        var ex = await Assert.ThrowsAsync<HttpRequestException>(() => Create(upstream).GetItemAsync(1));

        Assert.Equal(HttpStatusCode.NotFound, ex.StatusCode);
        Assert.Equal(1, upstream.Calls);
    }

    [Fact]
    public async Task A_hanging_attempt_times_out_and_is_retried()
    {
        var upstream = new ScriptedHandler(Hang(), Status(HttpStatusCode.OK, "widget"));

        Assert.Equal("widget", await Create(upstream, TimeSpan.FromMilliseconds(50)).GetItemAsync(1));
        Assert.Equal(2, upstream.Calls);
    }
}
