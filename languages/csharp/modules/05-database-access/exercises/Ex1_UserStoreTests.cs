using Microsoft.Data.Sqlite;

namespace M05.Exercises;

public class Ex1_UserStoreTests
{
    private const string Schema = "CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT NOT NULL, email TEXT NOT NULL UNIQUE, bio TEXT)";

    [Fact]
    public async Task Add_returns_the_user_with_its_id_and_keeps_null_and_empty_bio_apart()
    {
        await using var conn = TestDb.Open(Schema);

        var ada = await Ex1_UserStore.AddAsync(conn, "Ada", "ada@example.com", null);
        var grace = await Ex1_UserStore.AddAsync(conn, "Grace", "grace@example.com", "");

        Assert.Equal(1, ada.Id);
        Assert.Equal(2, grace.Id);
        Assert.Null((await Ex1_UserStore.FindByEmailAsync(conn, "ada@example.com"))!.Bio);
        Assert.Equal("", (await Ex1_UserStore.FindByEmailAsync(conn, "grace@example.com"))!.Bio);
        Assert.Equal(1, TestDb.Scalar(conn, "SELECT count(*) FROM users WHERE bio IS NULL"));
    }

    [Fact]
    public async Task Quotes_in_values_are_stored_literally()
    {
        await using var conn = TestDb.Open(Schema);

        await Ex1_UserStore.AddAsync(conn, "Robert'); DROP TABLE users;--", "bobby@example.com", "it's \"fine\"");

        var found = await Ex1_UserStore.FindByEmailAsync(conn, "bobby@example.com");
        Assert.Equal("Robert'); DROP TABLE users;--", found!.Name);
        Assert.Equal("it's \"fine\"", found.Bio);
    }

    [Theory]
    [InlineData("nobody@example.com")]
    [InlineData("x' OR '1'='1")]
    [InlineData("' OR 1=1 --")]
    public async Task Find_returns_null_for_unknown_and_hostile_input(string email)
    {
        await using var conn = TestDb.Open(Schema);
        await Ex1_UserStore.AddAsync(conn, "Ada", "ada@example.com", null);

        Assert.Null(await Ex1_UserStore.FindByEmailAsync(conn, email));
    }

    [Fact]
    public async Task Duplicate_email_surfaces_as_SqliteException()
    {
        await using var conn = TestDb.Open(Schema);
        await Ex1_UserStore.AddAsync(conn, "Ada", "ada@example.com", null);

        await Assert.ThrowsAsync<SqliteException>(() => Ex1_UserStore.AddAsync(conn, "Other", "ada@example.com", null));
    }

    [Fact]
    public async Task Search_treats_the_fragment_as_plain_text()
    {
        await using var conn = TestDb.Open(Schema);
        await Ex1_UserStore.AddAsync(conn, "Ada Lovelace", "ada@example.com", null);
        await Ex1_UserStore.AddAsync(conn, "Grace Hopper", "grace@example.com", null);
        await Ex1_UserStore.AddAsync(conn, "100% Real", "real@example.com", null);

        Assert.Equal(["Ada Lovelace"], (await Ex1_UserStore.SearchByNameAsync(conn, "LOVE")).Select(u => u.Name));
        Assert.Equal(["Ada Lovelace", "Grace Hopper", "100% Real"], (await Ex1_UserStore.SearchByNameAsync(conn, "a")).Select(u => u.Name));
        Assert.Equal(["Grace Hopper"], (await Ex1_UserStore.SearchByNameAsync(conn, "hopper")).Select(u => u.Name));
        Assert.Equal(["100% Real"], (await Ex1_UserStore.SearchByNameAsync(conn, "%")).Select(u => u.Name));
        Assert.Empty(await Ex1_UserStore.SearchByNameAsync(conn, "' OR '1'='1"));
    }
}
