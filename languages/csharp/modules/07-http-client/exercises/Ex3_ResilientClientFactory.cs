using Microsoft.Extensions.DependencyInjection;

namespace M07.Exercises;

public static class InventoryClientFactory
{
    /// <summary>
    /// Builds a service provider in which <see cref="InventoryClient"/> is registered as a typed client of
    /// <see cref="IHttpClientFactory"/> with:
    /// <list type="bullet">
    /// <item><c>BaseAddress = baseAddress</c> and the default request header <c>X-Api-Key: {apiKey}</c>;</item>
    /// <item><paramref name="primaryHandler"/> as the innermost transport (tests inject a fake);</item>
    /// <item>a resilience pipeline: retry at most 2 times (constant 1 ms delay, no jitter) on 5xx/408/429/network errors/attempt
    /// timeouts, with each attempt limited to <paramref name="attemptTimeout"/>. 4xx other than 408/429 is not retried.</item>
    /// </list>
    /// </summary>
    public static IServiceProvider Build(Uri baseAddress, string apiKey, Func<HttpMessageHandler> primaryHandler, TimeSpan attemptTimeout)
    {
        throw new NotImplementedException();
    }
}
