using System.Net;
using System.Net.Http.Json;
using System.Text;
using System.Text.Json;

namespace M03.Exercises;

public class Ex2_TaskValidationTests
{
    static CreateTaskRequest Valid() => new("Write docs", "todo", "2030-01-31", ["docs", "p1"], 7);

    static IReadOnlyDictionary<string, string[]> Check(CreateTaskRequest request) => TaskValidator.Validate(request);

    [Fact]
    public void AValidRequestHasNoErrors()
    {
        Assert.Empty(Check(Valid()));
    }

    [Fact]
    public void OptionalFieldsMayBeMissing()
    {
        Assert.Empty(Check(new CreateTaskRequest("Only a title", null, null, null, null)));
    }

    [Fact]
    public void TitleIsRequired()
    {
        var errors = Check(Valid() with { Title = null });

        Assert.Equal(["is required"], errors["title"]);
    }

    [Theory]
    [InlineData("")]
    [InlineData("    ")]
    public void BlankTitlesAreRejected(string title)
    {
        Assert.Equal(["must be 1-200 characters"], Check(Valid() with { Title = title })["title"]);
    }

    [Fact]
    public void TitleLengthIsMeasuredAfterTrimming()
    {
        Assert.Empty(Check(Valid() with { Title = "  " + new string('x', 200) + "  " }));
        Assert.Equal(["must be 1-200 characters"], Check(Valid() with { Title = new string('x', 201) })["title"]);
    }

    [Theory]
    [InlineData("todo", true)]
    [InlineData("in_progress", true)]
    [InlineData("done", true)]
    [InlineData("Done", false)]
    [InlineData("started", false)]
    [InlineData("", false)]
    public void StatusMustBeAKnownWireValue(string status, bool valid)
    {
        var errors = Check(Valid() with { Status = status });

        Assert.Equal(valid, !errors.ContainsKey("status"));
        if (!valid)
        {
            Assert.Equal(["must be one of: todo, in_progress, done"], errors["status"]);
        }
    }

    [Theory]
    [InlineData("2030-01-31", true)]
    [InlineData("2030-02-30", false)]
    [InlineData("31/01/2030", false)]
    [InlineData("2030-1-3", false)]
    [InlineData("tomorrow", false)]
    public void DueDateMustBeARealIsoDate(string due, bool valid)
    {
        var errors = Check(Valid() with { DueDate = due });

        Assert.Equal(valid, !errors.ContainsKey("due_date"));
        if (!valid)
        {
            Assert.Equal(["must be a date like 2030-01-31"], errors["due_date"]);
        }
    }

    [Fact]
    public void TagRulesAreCheckedAndEachViolationIsReported()
    {
        Assert.Equal(["at most 10 tags"], Check(Valid() with { Tags = [.. Enumerable.Range(0, 11).Select(i => $"t{i}")] })["tags"]);
        Assert.Equal(["each tag must be 1-30 characters"], Check(Valid() with { Tags = ["ok", ""] })["tags"]);
        Assert.Equal(["each tag must be 1-30 characters"], Check(Valid() with { Tags = [new string('a', 31)] })["tags"]);
        Assert.Equal(["every tag must be lowercase"], Check(Valid() with { Tags = ["Docs"] })["tags"]);
        Assert.Equal(["tags must be unique"], Check(Valid() with { Tags = ["a", "a"] })["tags"]);
        Assert.Equal(["every tag must be lowercase", "tags must be unique"], Check(Valid() with { Tags = ["A", "A"] })["tags"]);
    }

    [Fact]
    public void TenTagsAreFine()
    {
        Assert.Empty(Check(Valid() with { Tags = [.. Enumerable.Range(0, 10).Select(i => $"t{i}")] }));
    }

    [Fact]
    public void OwnerMustBePositive()
    {
        Assert.Equal(["must be positive"], Check(Valid() with { OwnerId = 0 })["owner_id"]);
        Assert.Empty(Check(Valid() with { OwnerId = 1 }));
    }

    [Fact]
    public void AllProblemsAreReportedAtOnce()
    {
        var errors = Check(new CreateTaskRequest(null, "x", "y", ["A"], -1));

        Assert.Equal(["due_date", "owner_id", "status", "tags", "title"], errors.Keys.Order());
    }

    static StringContent Json(string json) => new(json, Encoding.UTF8, "application/json");

    [Fact]
    public async Task InvalidBodiesGetA422ProblemDocument()
    {
        await using var app = await TestApp.StartAsync(a => TaskEndpoints.MapCreateTask(a));

        var response = await app.Client.PostAsync("/tasks", Json("""{"title":"","tags":["A"],"due_date":"nope"}"""));

        Assert.Equal(HttpStatusCode.UnprocessableEntity, response.StatusCode);
        Assert.Equal("application/problem+json", response.Content.Headers.ContentType?.MediaType);
        using var body = JsonDocument.Parse(await response.Content.ReadAsStringAsync());
        var errors = body.RootElement.GetProperty("errors");
        Assert.Equal("must be 1-200 characters", errors.GetProperty("title")[0].GetString());
        Assert.Equal("every tag must be lowercase", errors.GetProperty("tags")[0].GetString());
        Assert.Equal("must be a date like 2030-01-31", errors.GetProperty("due_date")[0].GetString());
    }

    [Fact]
    public async Task ValidBodiesGetA201WithLocation()
    {
        await using var app = await TestApp.StartAsync(a => TaskEndpoints.MapCreateTask(a));

        var response = await app.Client.PostAsync("/tasks", Json("""{"title":"  Write docs  ","due_date":"2030-01-31","owner_id":3}"""));

        Assert.Equal(HttpStatusCode.Created, response.StatusCode);
        Assert.Equal("/tasks/1", response.Headers.Location?.ToString());
        var created = await response.Content.ReadFromJsonAsync<JsonElement>();
        Assert.Equal("Write docs", created.GetProperty("title").GetString());
    }

    [Fact]
    public async Task MalformedJsonIsABadRequestNotAValidationError()
    {
        await using var app = await TestApp.StartAsync(a => TaskEndpoints.MapCreateTask(a));

        var response = await app.Client.PostAsync("/tasks", Json("""{"title":"""));

        Assert.Equal(HttpStatusCode.BadRequest, response.StatusCode);
    }
}
