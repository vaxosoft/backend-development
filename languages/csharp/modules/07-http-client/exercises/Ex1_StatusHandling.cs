#pragma warning disable CS9113 // starter: the constructor parameters are for your implementation to use

namespace M07.Exercises;

/// <summary>
/// Reads task titles from a Task Tracker API.
/// Contract for <see cref="GetTitleAsync"/>:
/// sends <c>GET tasks/{id}</c> with <c>Accept: application/json</c>;
/// 200 returns the JSON member <c>title</c>; 404 returns null;
/// any other non-2xx status throws <see cref="HttpRequestException"/> carrying that status code.
/// The response must be disposed on every path.
/// </summary>
public sealed class TaskTitleClient(HttpClient http)
{
    public Task<string?> GetTitleAsync(int id, CancellationToken cancellationToken = default)
    {
        throw new NotImplementedException();
    }
}
