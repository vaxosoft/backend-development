namespace M11.Exercises;

public class Ex1_PasswordHasherTests
{
    private readonly Ex1_PasswordHasher _hasher = new(1_000);

    [Fact]
    public void Hash_UsesSelfDescribingFormat()
    {
        var parts = _hasher.Hash("hunter2!!").Split('$');

        Assert.Equal(4, parts.Length);
        Assert.Equal("pbkdf2-sha256", parts[0]);
        Assert.Equal("1000", parts[1]);
        Assert.Equal(16, Convert.FromBase64String(parts[2]).Length);
        Assert.Equal(32, Convert.FromBase64String(parts[3]).Length);
    }

    [Fact]
    public void Hash_IsSaltedSoTheSamePasswordGivesDifferentRecords() =>
        Assert.NotEqual(_hasher.Hash("same"), _hasher.Hash("same"));

    [Fact]
    public void Verify_AcceptsOnlyTheOriginalPassword()
    {
        var stored = _hasher.Hash("correct horse");

        Assert.True(_hasher.Verify("correct horse", stored));
        Assert.False(_hasher.Verify("correct horsf", stored));
        Assert.False(_hasher.Verify("", stored));
    }

    [Theory]
    [InlineData("")]
    [InlineData("plaintext")]
    [InlineData("pbkdf2-sha256$abc$AAAA$AAAA")]
    [InlineData("pbkdf2-sha256$1000$!!notbase64!!$AAAA")]
    [InlineData("bcrypt$1000$AAAA$AAAA")]
    [InlineData("pbkdf2-sha256$1000$AAAA")]
    public void Verify_ReturnsFalseForMalformedRecords(string stored) =>
        Assert.False(_hasher.Verify("anything", stored));

    [Fact]
    public void Verify_UsesTheIterationCountStoredInTheRecord()
    {
        var oldRecord = new Ex1_PasswordHasher(1_000).Hash("pw");

        Assert.True(new Ex1_PasswordHasher(5_000).Verify("pw", oldRecord));
    }

    [Fact]
    public void NeedsRehash_FlagsWeakerOrBrokenRecords()
    {
        var weak = new Ex1_PasswordHasher(1_000).Hash("pw");
        var current = new Ex1_PasswordHasher(5_000);

        Assert.True(current.NeedsRehash(weak));
        Assert.False(current.NeedsRehash(current.Hash("pw")));
        Assert.True(current.NeedsRehash("garbage"));
    }

    [Fact]
    public void Hash_RejectsNullPassword() =>
        Assert.Throws<ArgumentNullException>(() => _hasher.Hash(null!));
}
