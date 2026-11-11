using System.Net;
using System.Net.Http.Headers;
using System.Text.Json;

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
    public async Task<string?> GetTitleAsync(int id, CancellationToken cancellationToken = default)
    {
        using var request = new HttpRequestMessage(HttpMethod.Get, $"tasks/{id}");
        request.Headers.Accept.Add(new MediaTypeWithQualityHeaderValue("application/json"));

        using var response = await http.SendAsync(request, cancellationToken);
        if (response.StatusCode == HttpStatusCode.NotFound)
        {
            return null;
        }

        response.EnsureSuccessStatusCode();
        using var json = await JsonDocument.ParseAsync(
            await response.Content.ReadAsStreamAsync(cancellationToken), cancellationToken: cancellationToken);
        return json.RootElement.GetProperty("title").GetString();
    }
}
