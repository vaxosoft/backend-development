# 04. Serialization in C#

> Estimated time: 1.5-2h | Difficulty: Intermediate | Prerequisites: [01 Language Fundamentals](../01-language-fundamentals/README.md), [03 REST API Design](../03-rest-api-design/README.md)

All commands run from `languages/csharp/`. Setup once per shell: `export DOTNET_NOLOGO=1 DOTNET_CLI_TELEMETRY_OPTOUT=1`.

## Learning Outcomes
After this module you can:
- [ ] Encode and decode JSON with `System.Text.Json`, with snake_case members and string enums matching the Task Tracker wire format.
- [ ] Control null, missing and unknown members (`required`, `JsonIgnore`, `UnmappedMemberHandling`) and tell *absent* from *null*.
- [ ] Write a `JsonConverter<T>` and format dates/timestamps as RFC 3339 on purpose.
- [ ] Generate serializers at compile time with `JsonSerializerContext`.
- [ ] Read and write CSV, YAML, XML and form-encoded data, and validate decoded data (400 vs 422).

## 1. Concept Overview
Serialization turns in-memory objects into bytes for a wire or a file (encode) and back (decode). Decoding is a trust boundary: the input comes from outside, so *malformed* input (not JSON, wrong types) is a client error (`400`), while *well-formed but invalid* data (empty title, 11 tags) is a validation error (`422`) and must be checked after decoding.

```
bytes --parse--> tokens --bind--> DTO --validate--> domain object
        400 malformed    400 wrong type   422 invalid values
```

Three details cause most bugs: how names map (`due_date` vs `DueDate`), how `null`/absent/unknown members behave, and how dates and enums look on the wire.

## 2. How C# Approaches It
| Need | Tool | Notes |
|------|------|-------|
| JSON | `System.Text.Json` (STJ), in the BCL | fast, UTF-8 native; Newtonsoft.Json is not needed |
| Names | `JsonSerializerOptions.PropertyNamingPolicy = JsonNamingPolicy.SnakeCaseLower`, `[JsonPropertyName]` | no per-field tags needed for a whole-API policy |
| Enums | `JsonStringEnumConverter(JsonNamingPolicy.SnakeCaseLower)` or `JsonStringEnumConverter<T>` + `[JsonStringEnumMemberName]` | the generic form is trim/AOT-safe |
| Custom shapes | `JsonConverter<T>` (`Utf8JsonReader`/`Utf8JsonWriter`) | |
| No-reflection | `JsonSerializerContext` + `[JsonSerializable]` source generator | needed for trimming/AOT (module 14) |
| Untyped | `JsonDocument` (read-only DOM), `JsonNode` (mutable) | |
| CSV / YAML | `CsvHelper` 34, `YamlDotNet` 18 (profile libraries) | not in the BCL |
| XML / forms | `System.Xml.Serialization.XmlSerializer`, `System.Web.HttpUtility.ParseQueryString` | BCL |

Idiomatic style: one shared `JsonSerializerOptions` (create it once and reuse it; options cache metadata), records for DTOs, `required` for mandatory members, and a source-generated context for the hot path. Avoid `JsonSerializer.Serialize(x)` with ad-hoc options in a loop, and avoid `dynamic`. ASP.NET Core (module 02) already uses `JsonSerializerDefaults.Web` (camelCase, case-insensitive); you will override the naming policy to get snake_case.

## 3. Guided Examples
Run an example with `dotnet run --project modules/04-serialization/examples -- <name>`.

### 3.1 JSON basics
- **Goal:** serialize/deserialize a record and see what options change.
- **Run:** `dotnet run --project modules/04-serialization/examples -- json-basics`
- **Code:** [examples/01_JsonBasics.cs](examples/01_JsonBasics.cs)
  ```csharp
  private static readonly JsonSerializerOptions Wire = new(JsonSerializerDefaults.Web)
  {
      PropertyNamingPolicy = JsonNamingPolicy.SnakeCaseLower,
      Converters = { new JsonStringEnumConverter(JsonNamingPolicy.SnakeCaseLower) },
  };
  ```
- **Walkthrough:** with no options STJ writes PascalCase names and the enum as a number (`"Status":1`). `Web` defaults make reading case-insensitive; the naming policy and enum converter give the wire format. Look at `created_at`: STJ writes a zero offset as `+00:00`, not `Z` (fixed in 3.3).
- **Expected output:**
  ```
  defaults : {"Id":1,"Title":"Write docs","Status":1,"DueDate":"2030-01-31","Tags":["docs","p1"],"OwnerId":null,"CreatedAt":"2030-01-01T09:00:00+00:00"}
  wire     : {"id":1,"title":"Write docs","status":"in_progress","due_date":"2030-01-31","tags":["docs","p1"],"owner_id":null,"created_at":"2030-01-01T09:00:00+00:00"}
  decoded  : id=2 status=Done due=null owner=7 created=2030-02-01T00:00:00.0000000+00:00
  anonymous: {"ok":true,"n":3}
  ```

### 3.2 Nulls, missing and unknown members
- **Goal:** decide what happens to what the client sends (or forgets).
- **Run:** `dotnet run --project modules/04-serialization/examples -- nulls-and-unknowns`
- **Code:** [examples/02_NullsAndUnknowns.cs](examples/02_NullsAndUnknowns.cs)
  ```csharp
  public required string Title { get; init; }
  [JsonPropertyName("due_date")] public DateOnly? DueDate { get; init; }
  [JsonIgnore] public string Internal { get; init; } = "never on the wire";
  // ... UnmappedMemberHandling = JsonUnmappedMemberHandling.Disallow
  ```
- **Walkthrough:** unknown members are ignored unless you opt into `Disallow`. A missing `required` member is a `JsonException`; a missing optional member silently keeps the C# default. `JsonIgnoreCondition.WhenWritingNull` omits nulls on output. To tell *absent* from *null* (PATCH!) you cannot use a DTO: read a `JsonDocument` and use `TryGetProperty` plus `ValueKind`.
- **Expected output:**
  ```
  unknown field ignored: Ship, due=null, tags=0, never on the wire
  missing required   : JsonException: JSON deserialization for type 'M04.Examples.NullsAndUnknowns+CreateTask' was missing required properties including: 'title'.
  unknown disallowed : JsonException: The JSON property 'extra' could not be mapped to any .NET member contained in type 'M04.Examples.NullsAndUnknowns+CreateTask'.
  bad date           : JsonException: The JSON value could not be converted to System.Nullable`1[System.DateOnly].
  wrong type         : JsonException: The JSON value could not be converted to System.String.
  malformed          : JsonException: Expected start of a property name or value, but instead reached end of data.
  {"title":"t"}
  {"title":"t","note":"n"}
  title    : value x
  due_date : null
  owner_id : absent
  ```

### 3.3 A custom converter
- **Goal:** take control of one type's JSON shape.
- **Run:** `dotnet run --project modules/04-serialization/examples -- custom-converter`
- **Code:** [examples/03_CustomConverter.cs](examples/03_CustomConverter.cs)
  ```csharp
  public override void Write(Utf8JsonWriter writer, DateTimeOffset value, JsonSerializerOptions options) =>
      writer.WriteStringValue(value.UtcDateTime.ToString("yyyy-MM-dd'T'HH:mm:ss'Z'", CultureInfo.InvariantCulture));
  ```
- **Walkthrough:** `UtcTimestampConverter` always writes UTC with `Z` and rejects other offsets on input. `TagsConverter` accepts `["a","b"]` *or* `"a, b"` but always writes an array: be liberal in what you read, strict in what you write. Throw `JsonException` from `Read`; STJ adds the JSON path to the message.
- **Expected output:**
  ```
  default STJ : "2030-01-01T11:00:00+02:00"
  converter   : "2030-01-01T09:00:00Z"
  task        : {"id":1,"title":"Write docs","status":"todo","due_date":null,"tags":["docs"],"owner_id":null,"created_at":"2030-01-01T09:00:00Z"}
  tags string : [api|db] due=2030-03-04
  rejected    : expected an RFC 3339 UTC timestamp such as 2030-01-01T09:00:00Z
  ```

### 3.4 Other formats
- **Goal:** the same task in CSV, YAML, XML and a form body.
- **Run:** `dotnet run --project modules/04-serialization/examples -- other-formats`
- **Code:** [examples/04_OtherFormats.cs](examples/04_OtherFormats.cs)
  ```csharp
  [CsvRecord]
  public sealed partial record TaskRow(int Id, string Title, string Status, [CsvColumn("due_date")] string? DueDate);
  using var csv = CsvWriter.Create(writer);  csv.WriteRecords(rows);
  ```
- **Walkthrough:** CsvHelper 34 is a rewrite: records are marked `[CsvRecord]` (source generated, hence `partial`) and readers/writers come from `Create`/`FromText`. YamlDotNet needs `UnderscoredNamingConvention` for snake_case. `XmlSerializer` is attribute driven; writing into a `StringWriter` declares `utf-16` because strings are UTF-16. Forms: `Uri.EscapeDataString` to write, `HttpUtility.ParseQueryString` to read (repeated keys via `GetValues`).
- **Expected output:**
  ```
  --- csv
  Id,Title,Status,due_date
  1,Write docs,todo,2030-01-31
  2,"Fix, escape ""quotes""",done,
  read back: TaskRow { Id = 1, Title = Write docs, Status = todo, DueDate = 2030-01-31 }
  read back: TaskRow { Id = 2, Title = Fix, escape "quotes", Status = done, DueDate =  }
  --- yaml
  title: Write docs
  status: in_progress
  tags:
  - docs
  - p1
  read back: From YAML / done / a,b
  --- xml
  <?xml version="1.0" encoding="utf-16"?>
  <task id="1">
    <title>Write docs</title>
    <tags>
      <tag>docs</tag>
      <tag>p1</tag>
    </tags>
  </task>
  read back: 7 x a
  --- form
  title=Fish%20%26%20chips&tag=a&tag=b&due_date=2030-01-31
  title=Fish & chips tags=a,b
  ```

### 3.5 Task Tracker: source-generated contract and validation
- **Goal:** a realistic request path: decode strictly, validate, answer 201 / 400 / 422.
- **Run:** `dotnet run --project modules/04-serialization/examples -- task-tracker`
- **Code:** [examples/05_TaskTrackerJson.cs](examples/05_TaskTrackerJson.cs)
  ```csharp
  [JsonSourceGenerationOptions(JsonSerializerDefaults.Web,
      PropertyNamingPolicy = JsonKnownNamingPolicy.SnakeCaseLower,
      UnmappedMemberHandling = JsonUnmappedMemberHandling.Disallow,
      DefaultIgnoreCondition = JsonIgnoreCondition.WhenWritingNull,
      Converters = [typeof(CustomConverter.UtcTimestampConverter)])]
  [JsonSerializable(typeof(NewTask))]
  internal sealed partial class WireContext : JsonSerializerContext;
  ```
- **Walkthrough:** `JsonException` means 400 (the message is replaced with `e.Path`, never echo parser internals). After a successful decode, `Validate` collects *all* field errors into a dictionary: 422 with `fields`. The result mirrors ASP.NET Core's `ValidationProblemDetails` (module 03) without needing a web host. `WhenWritingNull` is why `owner_id` is absent here but `null` in 3.1.
- **Expected output:**
  ```
  > {"title":"  Write docs  ","due_date":"2030-01-31","tags":["docs","p1"]}
  < 201 {"id":1,"title":"Write docs","status":"todo","due_date":"2030-01-31","tags":["docs","p1"],"created_at":"2030-01-01T09:00:00Z"}
  > {"title":"No tags, defaults apply"}
  < 201 {"id":2,"title":"No tags, defaults apply","status":"todo","tags":[],"created_at":"2030-01-01T09:00:00Z"}
  > {"title":"x",
  < {"status":400,"code":"bad_request","message":"invalid JSON at $"}
  > {"titel":"typo"}
  < {"status":400,"code":"bad_request","message":"invalid JSON at $.titel"}
  > {"title":"x","status":"finished"}
  < {"status":400,"code":"bad_request","message":"invalid JSON at $.status"}
  > {"title":"   ","tags":["Docs","docs","docs"]}
  < {"status":422,"code":"validation_failed","message":"invalid task","fields":{"title":"must be 1-200 characters","tags":"each tag must be 1-30 lowercase characters"}}
  ```

## 4. Common Pitfalls
| Mistake | Why it happens | Fix |
|---------|----------------|-----|
| `Deserialize` ignores `due_date` / returns defaults | raw `JsonSerializerOptions` is case-sensitive and PascalCase; names do not match | one shared options object with `PropertyNamingPolicy` (and `Web` defaults for case-insensitivity) |
| A missing optional member silently becomes `0`/`null` | STJ binds only what is present | mark mandatory members `required`; validate after decoding |
| Typos in client JSON are accepted | unknown members are skipped by default | `UnmappedMemberHandling = Disallow` where you want strictness |
| Timestamps come out as `+00:00` and enums as numbers | `DateTimeOffset` keeps its offset; enums default to numeric | converter that writes UTC with `Z`; `JsonStringEnumConverter` with `allowIntegerValues: false` |
| PATCH cannot tell `"due_date": null` from absent | both bind to `null` on a DTO | read a `JsonDocument` (or `Optional<T>` wrapper) and check `TryGetProperty` |
| Polymorphic JSON loses the subtype | STJ serializes the declared type | `[JsonPolymorphic]` + `[JsonDerivedType]`, never `object` |
| Trimmed/AOT publish throws at runtime | reflection-based metadata was trimmed | source-generated `JsonSerializerContext` (module 14) |
| Messages contain `\u0027` | the default encoder escapes `'`, `<`, `+` for HTML safety | fine for JSON bodies; only relax the encoder deliberately |
| Creating `new JsonSerializerOptions` per call | each instance rebuilds its metadata cache | `static readonly` options |
| CsvHelper code from old blogs does not compile | v34 replaced `CsvConfiguration`/`ClassMap` with `[CsvRecord]` and `CsvReader.Create` | read the v34 docs, not v33 samples |

## 5. Coming From Other Languages
- **Java/Jackson:** STJ has no `ObjectMapper`; options are immutable after first use. Annotations map to `[JsonPropertyName]`/`[JsonIgnore]`; `@JsonCreator` is the constructor or `required`/`init` members. Unknown properties are ignored by default (Jackson fails by default).
- **TypeScript/JS:** `JSON.parse` returns whatever is there; STJ binds to a type and throws `JsonException` on a type mismatch, so there is no unchecked `any`. Use `JsonDocument`/`JsonNode` for the untyped case.
- **Go:** struct tags become attributes or a naming policy; `omitempty` is `JsonIgnoreCondition`; `json.RawMessage` is `JsonElement`.
- **Python:** `json.loads` gives dicts; the typed counterpart is a record plus validation, similar to pydantic but with no built-in validation: you write it (or use DataAnnotations in ASP.NET).

## 6. Exercises
Starters and tests live in `exercises/` (project `M04.Exercises`); reference solutions in `solutions/` (same file names; copy a solution over its starter to compare).

| # | Title | Difficulty | Skill practiced | Run tests |
|---|-------|-----------|-----------------|-----------|
| 1 | Wire options | Easy | naming policy, string enums, strict enum reading | `dotnet test modules/04-serialization/exercises --filter "FullyQualifiedName~.Ex1_"` |
| 2 | Email converter | Medium | `JsonConverter<T>` for a value object, validation in `Read` | `dotnet test modules/04-serialization/exercises --filter "FullyQualifiedName~.Ex2_"` |
| 3 | PATCH decoder | Stretch | `JsonDocument`, absent vs null, collecting field errors | `dotnet test modules/04-serialization/exercises --filter "FullyQualifiedName~.Ex3_"` |

### Exercise 1: Wire options ([Ex1_Wire.cs](exercises/Ex1_Wire.cs))
- **Task:** implement `CreateOptions()` so `TaskDto` serializes to the Task Tracker wire format and reads case-insensitively.
- **Acceptance:** `Ex1_WireTests`: exact JSON string, case-insensitive read with unknown members ignored, round trip, and `"finished"` / `1` rejected as status.
- **Hint:** `JsonSerializerDefaults.Web` already covers reading; the enum converter has a constructor parameter that disallows numbers.

### Exercise 2: Email converter ([Ex2_EmailConverter.cs](exercises/Ex2_EmailConverter.cs))
- **Task:** implement `EmailConverter` so `Email` is a plain, trimmed, lower-cased string on the wire and invalid values fail at decode time.
- **Acceptance:** `Ex2_EmailConverterTests`: normalisation, plain-string output, and seven invalid tokens (`null`, numbers, arrays, bad shapes) all raise `JsonException`.
- **Hint:** check `reader.TokenType` before `GetString()`; throw `JsonException`, not `FormatException`.

### Exercise 3: PATCH decoder ([Ex3_Patch.cs](exercises/Ex3_Patch.cs))
- **Task:** `Apply(current, json)` merges a PATCH body: absent = unchanged, `due_date: null` clears, everything validated, all errors reported together, nothing applied on error.
- **Acceptance:** `Ex3_PatchTests`: merge semantics, malformed bodies flagged, 13 invalid inputs mapped to the right field key, errors collected, 11 tags rejected.
- **Hint:** `JsonDocument.Parse` then `foreach (var member in root.EnumerateObject())` with a `switch` on `member.Name`; keep a working copy and a dictionary of errors.

## 7. Recap
- Decode is a trust boundary: parse (400), bind (400), validate (422).
- One shared options object defines names, enums, dates and strictness; test the exact JSON.
- `required`, `UnmappedMemberHandling` and converters make decoding strict; `JsonDocument` distinguishes absent from null.
- Source generators remove reflection and are required for trimming/AOT.
- Reach for STJ first; CsvHelper/YamlDotNet/XmlSerializer only for those formats.

## 8. Self-check Questions
1. Why does `{"Due_Date": "2030-01-31"}` bind with `Web` defaults but not with raw `new JsonSerializerOptions()`, and which name does `SnakeCaseLower` expect?
2. A client sends `{"title":"x","extra":1}`. Which setting decides between accepting and rejecting it, and what status code would you return?
3. Why can a `TaskPatch` DTO with `DateOnly? DueDate` not implement `PATCH` correctly?
4. When would you choose a `JsonSerializerContext` over plain `JsonSerializer.Serialize<T>`?

## 9. Further Reading
- [System.Text.Json overview](https://learn.microsoft.com/dotnet/standard/serialization/system-text-json/overview) and [source generation](https://learn.microsoft.com/dotnet/standard/serialization/system-text-json/source-generation)
- [Custom converters](https://learn.microsoft.com/dotnet/standard/serialization/system-text-json/converters-how-to)
- [CsvHelper v34 docs](https://joshclose.github.io/CsvHelper/) and [YamlDotNet](https://github.com/aaubry/YamlDotNet)

---
**Next:** [05 Database Access](../05-database-access/README.md)
