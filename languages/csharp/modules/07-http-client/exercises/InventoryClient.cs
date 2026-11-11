namespace M07.Exercises;

/// <summary>Typed client used by exercise 3. Already complete: the exercise is how it is wired.</summary>
public sealed class InventoryClient(HttpClient http)
{
    public async Task<string> GetItemAsync(int id, CancellationToken cancellationToken = default)
    {
        using var response = await http.GetAsync($"items/{id}", cancellationToken);
        response.EnsureSuccessStatusCode();
        return await response.Content.ReadAsStringAsync(cancellationToken);
    }
}
