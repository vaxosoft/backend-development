namespace M01.Exercises;

public class Ex1_TagNormalizerTests
{
    [Fact]
    public void TrimsAndLowercases()
    {
        Assert.Equal(["docs", "p1"], TagNormalizer.Normalize(["  Docs", "P1 "]));
    }

    [Fact]
    public void DropsNullAndBlankEntries()
    {
        Assert.Equal(["a"], TagNormalizer.Normalize([null, "", "   ", "a"]));
    }

    [Fact]
    public void RemovesDuplicatesAfterNormalizingKeepingFirstPosition()
    {
        Assert.Equal(["b", "a", "c"], TagNormalizer.Normalize(["B", "a", " b ", "A", "c"]));
    }

    [Fact]
    public void EmptyInputGivesEmptyList()
    {
        Assert.Empty(TagNormalizer.Normalize([]));
    }

    [Fact]
    public void NullSourceThrows()
    {
        Assert.Throws<ArgumentNullException>(() => TagNormalizer.Normalize(null!));
    }

    [Fact]
    public void ReadsTheSourceExactlyOnceAndReturnsAMaterializedList()
    {
        var enumerations = 0;

        IEnumerable<string?> Source()
        {
            enumerations++;
            yield return "x";
            yield return "X";
        }

        var result = TagNormalizer.Normalize(Source());
        _ = result.Count;
        _ = result.ToList();

        Assert.Equal(1, enumerations);
        Assert.Equal(["x"], result);
    }
}
