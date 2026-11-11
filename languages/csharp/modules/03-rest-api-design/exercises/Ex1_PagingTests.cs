using System.Text.Json;

namespace M03.Exercises;

public class Ex1_PagingTests
{
    static PageRequest Create(int? limit, int? offset)
    {
        Assert.True(PageRequest.TryCreate(limit, offset, out var request, out var error), error);
        Assert.Null(error);
        return request;
    }

    [Fact]
    public void DefaultsToTwentyFromTheStart()
    {
        Assert.Equal(new PageRequest(20, 0), Create(null, null));
    }

    [Fact]
    public void KeepsValuesInRange()
    {
        Assert.Equal(new PageRequest(5, 10), Create(5, 10));
        Assert.Equal(new PageRequest(100, 0), Create(100, null));
    }

    [Theory]
    [InlineData(101)]
    [InlineData(100_000)]
    public void ClampsHugeLimitsInsteadOfFailing(int limit)
    {
        Assert.Equal(100, Create(limit, null).Limit);
    }

    [Theory]
    [InlineData(0, null, "limit")]
    [InlineData(-5, null, "limit")]
    [InlineData(10, -1, "offset")]
    public void RejectsInvalidValuesWithAMessage(int? limit, int? offset, string mentions)
    {
        Assert.False(PageRequest.TryCreate(limit, offset, out _, out var error));
        Assert.Contains(mentions, error);
    }

    [Fact]
    public void PageSlicesAndReportsTheTotal()
    {
        var page = Page<int>.Of(Enumerable.Range(1, 10), new PageRequest(3, 3));

        Assert.Equal([4, 5, 6], page.Items);
        Assert.Equal(10, page.Total);
        Assert.Equal(3, page.Limit);
        Assert.Equal(3, page.Offset);
    }

    [Fact]
    public void LastPageMayBeShortAndOffsetsPastTheEndGiveNoItems()
    {
        Assert.Equal([10], Page<int>.Of(Enumerable.Range(1, 10), new PageRequest(3, 9)).Items);

        var beyond = Page<int>.Of(Enumerable.Range(1, 10), new PageRequest(3, 50));
        Assert.Empty(beyond.Items);
        Assert.Equal(10, beyond.Total);
    }

    [Fact]
    public void PageEnumeratesTheSourceOnce()
    {
        var enumerations = 0;

        IEnumerable<int> Source()
        {
            enumerations++;
            yield return 1;
            yield return 2;
        }

        _ = Page<int>.Of(Source(), new PageRequest(1, 0));

        Assert.Equal(1, enumerations);
    }

    [Fact]
    public void PageSerializesToTheListEnvelope()
    {
        var json = JsonSerializer.Serialize(Page<int>.Of([1, 2, 3], new PageRequest(2, 0)), JsonSerializerOptions.Web);

        Assert.Equal("""{"items":[1,2],"total":3,"limit":2,"offset":0}""", json);
    }

    static readonly IReadOnlySet<string> Allowed = new HashSet<string> { "created_at", "due_date" };

    [Theory]
    [InlineData("due_date", "due_date", false)]
    [InlineData("-due_date", "due_date", true)]
    [InlineData(null, "created_at", true)]
    [InlineData("", "created_at", true)]
    public void ParsesSortSpecs(string? raw, string field, bool descending)
    {
        Assert.True(SortSpec.TryParse(raw, "-created_at", Allowed, out var spec));
        Assert.Equal(new SortSpec(field, descending), spec);
    }

    [Theory]
    [InlineData("title")]
    [InlineData("-title")]
    [InlineData("--due_date")]
    [InlineData("due_date; drop table tasks")]
    public void RejectsSortFieldsThatAreNotAllowListed(string raw)
    {
        Assert.False(SortSpec.TryParse(raw, "-created_at", Allowed, out _));
    }
}
