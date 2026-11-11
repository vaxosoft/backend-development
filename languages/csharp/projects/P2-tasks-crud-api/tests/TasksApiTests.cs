using System.Net;
using System.Text.Json;

namespace P2.TasksCrudApi.Tests;

public sealed class TasksApiTests : IDisposable
{
    private readonly TasksApi _api = new();

    public void Dispose() => _api.Dispose();

    private static string TitleOnly(string title) => $$"""{"title":"{{title}}"}""";

    // --- create -----------------------------------------------------------

    [Fact]
    public async Task Create_Returns201WithLocationAndTheStoredTask()
    {
        var response = await _api.PostAsync(TitleOnly("Write docs"));

        Assert.Equal(HttpStatusCode.Created, response.StatusCode);
        var body = await TasksApi.Body(response);
        var id = body.GetProperty("id").GetInt64();
        Assert.Equal($"/tasks/{id}", response.Headers.Location?.OriginalString);
        Assert.Equal("Write docs", body.GetProperty("title").GetString());
        Assert.Equal("2030-01-01T09:00:00Z", body.GetProperty("created_at").GetString());
    }

    [Fact]
    public async Task Create_AppliesDefaults()
    {
        var body = await TasksApi.Body(await _api.PostAsync(TitleOnly("Defaults")));

        Assert.Equal("todo", body.GetProperty("status").GetString());
        Assert.Equal(JsonValueKind.Null, body.GetProperty("due_date").ValueKind);
        Assert.Equal(JsonValueKind.Null, body.GetProperty("owner_id").ValueKind);
        Assert.Equal(0, body.GetProperty("tags").GetArrayLength());
    }

    [Fact]
    public async Task Create_StoresAllFieldsAndTrimsTheTitle()
    {
        var json = """{"title":"  Ship it  ","status":"in_progress","due_date":"2030-01-31","tags":["docs","p1"],"owner_id":7}""";

        var body = await TasksApi.Body(await _api.PostAsync(json));

        Assert.Equal("Ship it", body.GetProperty("title").GetString());
        Assert.Equal("in_progress", body.GetProperty("status").GetString());
        Assert.Equal("2030-01-31", body.GetProperty("due_date").GetString());
        Assert.Equal(["docs", "p1"], body.GetProperty("tags").EnumerateArray().Select(t => t.GetString()));
        Assert.Equal(7, body.GetProperty("owner_id").GetInt64());
    }

    [Theory]
    [InlineData("""{"title":""}""", "title")]
    [InlineData("""{"title":"   "}""", "title")]
    [InlineData("""{}""", "title")]
    [InlineData("""{"title":"ok","status":"finished"}""", "status")]
    [InlineData("""{"title":"ok","due_date":"2030-02-30"}""", "due_date")]
    [InlineData("""{"title":"ok","due_date":"31/01/2030"}""", "due_date")]
    [InlineData("""{"title":"ok","tags":["Docs"]}""", "tags")]
    [InlineData("""{"title":"ok","tags":["a","a"]}""", "tags")]
    [InlineData("""{"title":"ok","tags":[""]}""", "tags")]
    [InlineData("""{"title":"ok","tags":["a","b","c","d","e","f","g","h","i","j","k"]}""", "tags")]
    public async Task Create_WithInvalidData_Returns422NamingTheField(string json, string field)
    {
        var response = await _api.PostAsync(json);

        Assert.Equal(HttpStatusCode.UnprocessableEntity, response.StatusCode);
        Assert.Equal("application/problem+json", response.Content.Headers.ContentType?.MediaType);
        var errors = (await TasksApi.Body(response)).GetProperty("errors");
        Assert.True(errors.TryGetProperty(field, out _), $"expected an error for '{field}', got {errors}");
    }

    [Theory]
    [InlineData(200, HttpStatusCode.Created)]
    [InlineData(201, HttpStatusCode.UnprocessableEntity)]
    public async Task Create_EnforcesTheTitleLengthLimit(int length, HttpStatusCode expected)
    {
        var response = await _api.PostAsync(TitleOnly(new string('x', length)));

        Assert.Equal(expected, response.StatusCode);
    }

    [Theory]
    [InlineData("{\"title\":")]
    [InlineData("not json")]
    [InlineData("""{"title":5}""")]
    public async Task Create_WithMalformedJson_Returns400(string json)
    {
        var response = await _api.PostAsync(json);

        Assert.Equal(HttpStatusCode.BadRequest, response.StatusCode);
        Assert.Equal("application/problem+json", response.Content.Headers.ContentType?.MediaType);
    }

    // --- read -------------------------------------------------------------

    [Fact]
    public async Task Get_ReturnsTheTask()
    {
        var id = await _api.CreateAsync("""{"title":"Find me","tags":["x"]}""");

        var response = await _api.CreateClient().GetAsync($"/tasks/{id}");

        Assert.Equal(HttpStatusCode.OK, response.StatusCode);
        var body = await TasksApi.Body(response);
        Assert.Equal("Find me", body.GetProperty("title").GetString());
        Assert.Equal(id, body.GetProperty("id").GetInt64());
    }

    [Theory]
    [InlineData("GET")]
    [InlineData("PUT")]
    [InlineData("PATCH")]
    [InlineData("DELETE")]
    public async Task UnknownId_Returns404ProblemJson(string method)
    {
        var response = await _api.SendJsonAsync(new HttpMethod(method), "/tasks/999", """{"title":"x"}""");

        Assert.Equal(HttpStatusCode.NotFound, response.StatusCode);
        Assert.Equal("application/problem+json", response.Content.Headers.ContentType?.MediaType);
    }

    // --- replace and patch ----------------------------------------------

    [Fact]
    public async Task Put_ReplacesEveryFieldAndKeepsCreatedAt()
    {
        var id = await _api.CreateAsync("""{"title":"Old","status":"done","due_date":"2030-05-05","tags":["old"],"owner_id":1}""");
        _api.Clock.Advance(TimeSpan.FromHours(1));

        var response = await _api.SendJsonAsync(HttpMethod.Put, $"/tasks/{id}", """{"title":"New"}""");

        Assert.Equal(HttpStatusCode.OK, response.StatusCode);
        var body = await TasksApi.Body(response);
        Assert.Equal("New", body.GetProperty("title").GetString());
        Assert.Equal("todo", body.GetProperty("status").GetString());
        Assert.Equal(JsonValueKind.Null, body.GetProperty("due_date").ValueKind);
        Assert.Equal(0, body.GetProperty("tags").GetArrayLength());
        Assert.Equal(JsonValueKind.Null, body.GetProperty("owner_id").ValueKind);
        Assert.Equal("2030-01-01T09:00:00Z", body.GetProperty("created_at").GetString());
    }

    [Fact]
    public async Task Put_WithInvalidData_Returns422AndChangesNothing()
    {
        var id = await _api.CreateAsync(TitleOnly("Keep"));

        var response = await _api.SendJsonAsync(HttpMethod.Put, $"/tasks/{id}", """{"title":""}""");

        Assert.Equal(HttpStatusCode.UnprocessableEntity, response.StatusCode);
        Assert.Equal(["Keep"], await _api.TitlesAsync());
    }

    [Fact]
    public async Task Patch_ChangesOnlyTheProvidedFields()
    {
        var id = await _api.CreateAsync("""{"title":"Before","due_date":"2030-05-05","tags":["a"],"owner_id":3}""");

        var response = await _api.SendJsonAsync(HttpMethod.Patch, $"/tasks/{id}", """{"status":"done"}""");

        Assert.Equal(HttpStatusCode.OK, response.StatusCode);
        var body = await TasksApi.Body(response);
        Assert.Equal("done", body.GetProperty("status").GetString());
        Assert.Equal("Before", body.GetProperty("title").GetString());
        Assert.Equal("2030-05-05", body.GetProperty("due_date").GetString());
        Assert.Equal(["a"], body.GetProperty("tags").EnumerateArray().Select(t => t.GetString()));
        Assert.Equal(3, body.GetProperty("owner_id").GetInt64());
    }

    [Fact]
    public async Task Patch_WithNullDueDate_ClearsIt()
    {
        var id = await _api.CreateAsync("""{"title":"Dated","due_date":"2030-05-05"}""");

        var body = await TasksApi.Body(await _api.SendJsonAsync(HttpMethod.Patch, $"/tasks/{id}", """{"due_date":null}"""));

        Assert.Equal(JsonValueKind.Null, body.GetProperty("due_date").ValueKind);
        Assert.Equal("Dated", body.GetProperty("title").GetString());
    }

    [Fact]
    public async Task Patch_IgnoresFieldsItDoesNotOwn()
    {
        var id = await _api.CreateAsync(TitleOnly("Mine"));

        var body = await TasksApi.Body(await _api.SendJsonAsync(HttpMethod.Patch, $"/tasks/{id}", """{"id":99,"owner_id":5,"created_at":"1999-01-01T00:00:00Z"}"""));

        Assert.Equal(id, body.GetProperty("id").GetInt64());
        Assert.Equal(JsonValueKind.Null, body.GetProperty("owner_id").ValueKind);
        Assert.Equal("2030-01-01T09:00:00Z", body.GetProperty("created_at").GetString());
    }

    [Theory]
    [InlineData("""{"title":""}""", "title")]
    [InlineData("""{"title":null}""", "title")]
    [InlineData("""{"status":"nope"}""", "status")]
    [InlineData("""{"status":null}""", "status")]
    [InlineData("""{"due_date":"tomorrow"}""", "due_date")]
    [InlineData("""{"tags":["UPPER"]}""", "tags")]
    public async Task Patch_WithInvalidValue_Returns422(string json, string field)
    {
        var id = await _api.CreateAsync(TitleOnly("Stable"));

        var response = await _api.SendJsonAsync(HttpMethod.Patch, $"/tasks/{id}", json);

        Assert.Equal(HttpStatusCode.UnprocessableEntity, response.StatusCode);
        Assert.True((await TasksApi.Body(response)).GetProperty("errors").TryGetProperty(field, out _));
    }

    // --- delete -----------------------------------------------------------

    [Fact]
    public async Task Delete_Returns204ThenTheTaskIsGone()
    {
        var id = await _api.CreateAsync(TitleOnly("Doomed"));
        var client = _api.CreateClient();

        var deleted = await client.DeleteAsync($"/tasks/{id}");
        var fetched = await client.GetAsync($"/tasks/{id}");

        Assert.Equal(HttpStatusCode.NoContent, deleted.StatusCode);
        Assert.Equal(HttpStatusCode.NotFound, fetched.StatusCode);
    }

    // --- list -------------------------------------------------------------

    [Fact]
    public async Task List_ReturnsTheEnvelopeWithDefaults()
    {
        await _api.CreateAsync(TitleOnly("One"));
        await _api.CreateAsync(TitleOnly("Two"));

        var list = await _api.ListAsync();

        Assert.Equal(2, list.GetProperty("total").GetInt32());
        Assert.Equal(20, list.GetProperty("limit").GetInt32());
        Assert.Equal(0, list.GetProperty("offset").GetInt32());
        Assert.Equal(2, list.GetProperty("items").GetArrayLength());
    }

    private async Task SeedThreeAsync()
    {
        await _api.CreateAsync("""{"title":"first","status":"todo","tags":["a","b"],"due_date":"2030-03-01"}""");
        _api.Clock.Advance(TimeSpan.FromMinutes(1));
        await _api.CreateAsync("""{"title":"second","status":"done","tags":["b"]}""");
        _api.Clock.Advance(TimeSpan.FromMinutes(1));
        await _api.CreateAsync("""{"title":"third","status":"todo","tags":["c"],"due_date":"2030-02-01"}""");
    }

    [Fact]
    public async Task List_DefaultsToNewestFirst()
    {
        await SeedThreeAsync();

        Assert.Equal(["third", "second", "first"], await _api.TitlesAsync());
    }

    [Fact]
    public async Task List_SortCreatedAt_OldestFirst()
    {
        await SeedThreeAsync();

        Assert.Equal(["first", "second", "third"], await _api.TitlesAsync("?sort=created_at"));
        Assert.Equal(["third", "second", "first"], await _api.TitlesAsync("?sort=-created_at"));
    }

    [Fact]
    public async Task List_SortDueDate_PutsTasksWithoutDueDateLast()
    {
        await SeedThreeAsync();

        Assert.Equal(["third", "first", "second"], await _api.TitlesAsync("?sort=due_date"));
        Assert.Equal(["first", "third", "second"], await _api.TitlesAsync("?sort=-due_date"));
    }

    [Fact]
    public async Task List_FiltersByStatus()
    {
        await SeedThreeAsync();

        var list = await _api.ListAsync("?status=todo&sort=created_at");

        Assert.Equal(2, list.GetProperty("total").GetInt32());
        Assert.Equal(["first", "third"], await _api.TitlesAsync("?status=todo&sort=created_at"));
    }

    [Fact]
    public async Task List_FiltersByTag()
    {
        await SeedThreeAsync();

        Assert.Equal(["second", "first"], await _api.TitlesAsync("?tag=b"));
        Assert.Empty(await _api.TitlesAsync("?tag=zzz"));
    }

    [Fact]
    public async Task List_PaginatesAndReportsTheUnpagedTotal()
    {
        await SeedThreeAsync();

        var list = await _api.ListAsync("?limit=1&offset=1");

        Assert.Equal(3, list.GetProperty("total").GetInt32());
        Assert.Equal(1, list.GetProperty("limit").GetInt32());
        Assert.Equal(1, list.GetProperty("offset").GetInt32());
        Assert.Equal(["second"], await _api.TitlesAsync("?limit=1&offset=1"));
    }

    [Fact]
    public async Task List_ClampsLimitAbove100()
    {
        var list = await _api.ListAsync("?limit=1000");

        Assert.Equal(100, list.GetProperty("limit").GetInt32());
    }

    [Theory]
    [InlineData("limit=abc")]
    [InlineData("limit=0")]
    [InlineData("offset=-1")]
    [InlineData("status=bogus")]
    [InlineData("sort=title")]
    public async Task List_WithBadQueryValues_Returns400(string query)
    {
        var response = await _api.CreateClient().GetAsync("/tasks?" + query);

        Assert.Equal(HttpStatusCode.BadRequest, response.StatusCode);
        Assert.Equal("application/problem+json", response.Content.Headers.ContentType?.MediaType);
    }

    // --- built-ins ---------------------------------------------------------

    [Fact]
    public async Task OpenApiDocument_IsServed()
    {
        var response = await _api.CreateClient().GetAsync("/openapi.json");

        Assert.Equal(HttpStatusCode.OK, response.StatusCode);
        Assert.Contains("/tasks", await response.Content.ReadAsStringAsync());
    }
}
