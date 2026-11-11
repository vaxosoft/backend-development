using System.Text.Json;

namespace M04.Exercises;

public class Ex2_EmailConverterTests
{
    private static readonly JsonSerializerOptions Options = new(JsonSerializerDefaults.Web)
    {
        PropertyNamingPolicy = JsonNamingPolicy.SnakeCaseLower,
    };

    [Fact]
    public void Reads_trimmed_lower_case_email()
    {
        var user = JsonSerializer.Deserialize<UserDto>("""{"id":1,"name":"Ada","email":"  Ada@Example.COM "}""", Options)!;

        Assert.Equal(new Email("ada@example.com"), user.Email);
    }

    [Fact]
    public void Writes_a_plain_string()
    {
        var json = JsonSerializer.Serialize(new UserDto(1, "Ada", new Email("ada@example.com")), Options);

        Assert.Equal("""{"id":1,"name":"Ada","email":"ada@example.com"}""", json);
    }

    [Theory]
    [InlineData("\"not-an-email\"")]
    [InlineData("\"@host\"")]
    [InlineData("\"a@\"")]
    [InlineData("\"a@b@c\"")]
    [InlineData("null")]
    [InlineData("42")]
    [InlineData("[\"a@b\"]")]
    public void Rejects_invalid_values_with_JsonException(string emailToken)
    {
        var json = $$"""{"id":1,"name":"Ada","email":{{emailToken}}}""";

        Assert.Throws<JsonException>(() => JsonSerializer.Deserialize<UserDto>(json, Options));
    }
}
