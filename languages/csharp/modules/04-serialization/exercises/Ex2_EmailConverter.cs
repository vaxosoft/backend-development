using System.Text.Json;
using System.Text.Json.Serialization;

namespace M04.Exercises;

/// <summary>A validated, lower-cased e-mail address.</summary>
[JsonConverter(typeof(EmailConverter))]
public readonly record struct Email(string Value);

public sealed record UserDto(int Id, string Name, Email Email);

/// <summary>
/// Reads a JSON string, trims and lower-cases it, and rejects anything without exactly one '@'
/// surrounded by text (JsonException). Writes the plain string. A JSON null or a non-string token is a JsonException too.
/// </summary>
public sealed class EmailConverter : JsonConverter<Email>
{
    public override Email Read(ref Utf8JsonReader reader, Type typeToConvert, JsonSerializerOptions options) =>
        throw new NotImplementedException();

    public override void Write(Utf8JsonWriter writer, Email value, JsonSerializerOptions options) =>
        throw new NotImplementedException();
}
