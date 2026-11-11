# 04. Serialization in Go

> Estimated time: 1.5-2h | Difficulty: Intermediate | Prerequisites: 01, 02, 03

## Learning Outcomes
After this module you can:
- [ ] Encode and decode JSON with struct tags and know exactly what `omitempty`, `omitzero`, `-` and unexported fields do
- [ ] Distinguish an absent key from `null` from a value (pointers vs a tri-state `Optional[T]`)
- [ ] Write `MarshalJSON`/`UnmarshalJSON` and `MarshalText`/`UnmarshalText` for dates and enums
- [ ] Decode request bodies strictly (unknown fields, trailing data, size limit) and validate the result
- [ ] Read and write XML, CSV, YAML and form-encoded data

## 1. Concept Overview
Serialization turns in-memory values into bytes for the wire and back. The wire format is a contract: field names, null handling and date formats must be identical for every client, regardless of how the server stores things. Decoded data is untrusted input, so decoding (syntax and types) is followed by validation (business rules), and the two failures map to different HTTP statuses (400 vs 422, see module 03).

The Task Tracker wire format (`snake_case`, `due_date` as `YYYY-MM-DD` or `null`, `tags` always an array, RFC 3339 timestamps) is used throughout.

## 2. How Go Approaches It

| Need | Tool |
|------|------|
| JSON | `encoding/json`: `Marshal`/`Unmarshal` for values, `Encoder`/`Decoder` for streams (`http.Request.Body`) |
| Field mapping | struct tags: `` `json:"due_date,omitempty"` `` (reflection-based, so only **exported** fields) |
| Custom JSON | `json.Marshaler` / `json.Unmarshaler`; for string-like values `encoding.TextMarshaler` / `TextUnmarshaler` (also works for map keys) |
| Deferred decoding | `json.RawMessage` |
| Strictness | `Decoder.DisallowUnknownFields`, `http.MaxBytesReader` |
| XML / CSV / form | `encoding/xml`, `encoding/csv`, `net/url` (`url.Values`) |
| YAML | `gopkg.in/yaml.v3` (not in the stdlib) |

Idiom: define **separate request and response types** (DTOs) instead of tagging your domain model for every client, and validate explicitly with plain functions that return a `map[string]string` of field errors. There is no annotation-driven validation in the stdlib and the lessons do not add a validator library.

## 3. Guided Examples

### 3.1 JSON basics
- **Goal:** tags, `omitempty` vs `omitzero`, `nil` vs empty slice, `any` decoding.
- **Run:** `go run ./modules/04-serialization/examples/01_json_basics`
- **Code:** [examples/01_json_basics/main.go](examples/01_json_basics/main.go)
- **Walkthrough:** `Priority int` with `omitempty` vanishes when it is `0`; `omitzero` (Go 1.24+) omits the zero value using `IsZero`, which is what you want for `bool`, `time.Time` and structs. A `nil` slice is `null`, `[]string{}` is `[]`. Decoding is case-insensitive and ignores unknown keys by default. Into `any`, every number is a `float64` (note the lost precision).
- **Expected output:**
  ```
  {"id":1,"title":"Write docs","status":"todo","tags":null}
  {"id":0,"title":"","status":"","tags":null}
  {"id":0,"title":"","status":"","tags":[]}
  {
    "id": 2,
    "title": "Ship",
    "status": "",
    "tags": [
      "p1"
    ]
  }
  {ID:7 Title:Loose Status: Tags:[] Priority:0 Done:false Notes: secret:}
  float64 9.007199254740992e+15
  json: cannot unmarshal string into Go struct field Task.id of type int
  ```

### 3.2 Absent vs null vs value
- **Goal:** model PATCH semantics (`due_date: null` clears, a missing key means "unchanged").
- **Run:** `go run ./modules/04-serialization/examples/02_null_and_optional`
- **Code:** [examples/02_null_and_optional/main.go](examples/02_null_and_optional/main.go)
  ```go
  type Optional[T any] struct{ Set, Null bool; Value T }

  func (o *Optional[T]) UnmarshalJSON(data []byte) error {
      o.Set = true
      if string(data) == "null" { o.Null = true; return nil }
      return json.Unmarshal(data, &o.Value)
  }
  ```
- **Walkthrough:** with `*string`, `{}` and `{"due_date":null}` both give `nil`. `encoding/json` calls `UnmarshalJSON` only when the key exists, and also for `null` when the field is not a pointer, so `Set` and `Null` capture all three states.
- **Expected output:**
  ```
  {"title":"a","due_date":"2030-01-31"}
  {"title":"b","due_date":null}
  {"title":"c"}
  pointer  {}                         nil=true
  pointer  {"due_date":null}          nil=true
  pointer  {"due_date":"2030-02-01"}  nil=false
  optional {}                         set=false null=false value=""
  optional {"due_date":null}          set=true null=true value=""
  optional {"due_date":"2030-02-01"}  set=true null=false value="2030-02-01"
  ```

### 3.3 Custom types: dates, enums, RawMessage
- **Goal:** control the wire representation of a type.
- **Run:** `go run ./modules/04-serialization/examples/03_custom_types`
- **Code:** [examples/03_custom_types/main.go](examples/03_custom_types/main.go)
- **Walkthrough:** `Date` embeds `time.Time` but overrides both JSON methods (value receiver for `Marshal`, pointer receiver for `Unmarshal`). `Status` is an `int` internally and a string on the wire via `MarshalText`/`UnmarshalText`; the same methods make it a valid map key. `json.RawMessage` keeps a payload as bytes until the `type` field says how to decode it. `time.Time` already emits RFC 3339.
- **Expected output:**
  ```
  {"title":"Write docs","status":"in_progress","due_date":"2030-01-31","created_at":"2030-01-01T09:00:00Z"}
  true 2030-01-31 true
  unknown status "later"
  due_date must be YYYY-MM-DD: parsing time "31/01/2030" as "2006-01-02": cannot parse "31/01/2030" as "2006"
  due_date must be a string: json: cannot unmarshal number into Go value of type string
  {"done":1,"todo":3}
  task.done 4 {"id":4}
  ```

### 3.4 XML, CSV, YAML, form-encoded
- **Goal:** the same task in four other formats.
- **Run:** `go run ./modules/04-serialization/examples/04_other_formats`
- **Code:** [examples/04_other_formats/main.go](examples/04_other_formats/main.go)
- **Walkthrough:** XML uses `xml:"id,attr"` and `xml:"tags>tag"` for wrapper elements; `csv.Writer` buffers, so call `Flush` and check `Error()`; YAML has its own tags (`yaml:"..."`) and indents sequences by four spaces; `url.Values.Encode` sorts keys and escapes.
- **Expected output:**
  ```
  <?xml version="1.0" encoding="UTF-8"?>
  <task id="1">
    <title>Write docs &amp; ship</title>
    <status>todo</status>
    <tags>
      <tag>docs</tag>
      <tag>p1</tag>
    </tags>
  </task>
  Write docs & ship [docs p1]
  id,title,tags
  1,"Write docs, ship",docs;p1
  2 Write docs, ship
  id: 1
  title: Write docs & ship
  status: todo
  tags:
      - docs
      - p1
  2 From YAML done true
  tag=home&tag=errands&title=Buy+milk+%26+eggs
  Buy milk & eggs [home errands]
  ```

### 3.5 Task Tracker: strict decode, validate, error envelope
- **Goal:** the complete request path `bytes -> struct -> validation -> 201 / 400 / 413 / 422`.
- **Run:** `go run ./modules/04-serialization/examples/05_decode_task` (uses `httptest.NewRecorder`; nothing listens)
- **Code:** [examples/05_decode_task/main.go](examples/05_decode_task/main.go)
  ```go
  dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBytes))
  dec.DisallowUnknownFields()
  if err := dec.Decode(dst); err != nil { return err }
  if _, err := dec.Token(); !errors.Is(err, io.EOF) { return errors.New("body must contain a single JSON value") }
  ```
- **Walkthrough:** `errors.As` classifies decode errors (`*json.SyntaxError`, `*json.UnmarshalTypeError`, `*http.MaxBytesError`) into 400/413 without leaking internals; `validate` trims and checks rules and returns all field errors at once (422).
- **Expected output:**
  ```
  valid           201 {"id":1,"title":"Write docs","status":"todo","due_date":"2030-01-31","tags":["docs"],"owner_id":null,"created_at":"2030-01-01T09:00:00Z"}
  invalid fields  422 {"error":{"code":"validation_failed","message":"invalid task","fields":{"due_date":"must be YYYY-MM-DD","status":"must be one of todo, in_progress, done","title":"must be 1-200 characters"}}}
  malformed       400 {"error":{"code":"bad_request","message":"malformed JSON"}}
  empty body      400 {"error":{"code":"bad_request","message":"malformed JSON"}}
  unknown field   400 {"error":{"code":"bad_request","message":"json: unknown field \"colour\""}}
  wrong type      400 {"error":{"code":"bad_request","message":"field title must be a string"}}
  trailing data   400 {"error":{"code":"bad_request","message":"body must contain a single JSON value"}}
  too large       413 {"error":{"code":"payload_too_large","message":"request body too large"}}
  ```

## 4. Common Pitfalls

| Mistake | Why it happens | Fix |
|---------|----------------|-----|
| Field missing from the output | Unexported (lowercase) fields are invisible to `encoding/json`; no error is raised | Export the field and tag it |
| `priority: 0` or `done: false` disappears | `omitempty` drops every zero value | Use a pointer, or `omitzero` when zero really means "absent" |
| `"tags": null` instead of `[]` | A `nil` slice/map encodes as `null` | Initialise with `[]T{}` before encoding |
| Large ids change after decoding | Decoding into `any` gives `float64` | Decode into a typed struct, or `Decoder.UseNumber` |
| `PATCH` cannot clear a field | `*T` is `nil` for both absent and `null` | Tri-state `Optional[T]` (3.2) |
| Typos in client JSON are silently ignored | Default decoder drops unknown keys and matches case-insensitively | `DisallowUnknownFields` for request bodies |
| `{"a":1} garbage` accepted | `Decode` reads one value and stops | Check that the next `Token()` is `io.EOF` |
| `UnmarshalJSON` on a value receiver does nothing | The method mutates a copy | Pointer receiver for `Unmarshal*`, value receiver for `Marshal*` |
| Time zone surprises in dates | `time.Parse` returns UTC, `time.Now()` is local | Parse and store dates as UTC; format with the fixed layout `2006-01-02` |
| Unbounded request bodies | `json.NewDecoder(r.Body)` reads as much as the client sends | Wrap with `http.MaxBytesReader` |
| CSV file truncated | `csv.Writer` buffers; forgetting `Flush` loses the tail | `w.Flush()` then `w.Error()` |

## 5. Coming From Other Languages
- **Java/C# (Jackson, System.Text.Json):** no annotations processor or attributes with logic; tags are plain strings read by reflection at runtime, and there is no built-in validation framework, so validation is ordinary code.
- **Python/JS:** `json.loads` returns dicts/objects; in Go prefer a typed struct and let the decoder enforce types. `map[string]any` is the escape hatch, not the default.
- **Rust (serde):** serde checks everything at compile time and rejects unknown fields per type with one attribute; Go decides at runtime and ignores unknown fields unless asked.

## 6. Exercises
Starter code and tests are in [exercises/](exercises/); reference solutions in [_solutions/](_solutions/).

| # | Title | Difficulty | Skill practiced | Run tests |
|---|-------|-----------|-----------------|-----------|
| 1 | JSON tags | Easy | tags, `-`, nil slice, null pointers | `go test ./modules/04-serialization/exercises/ex1_json_tags/` |
| 2 | Date and enum types | Medium | `MarshalJSON`/`UnmarshalJSON`, `TextMarshaler` | `go test ./modules/04-serialization/exercises/ex2_date_type/` |
| 3 | Strict PATCH | Stretch | tri-state `Optional[T]`, strict decoding, validation | `go test ./modules/04-serialization/exercises/ex3_patch/` |

### Exercise 1: JSON tags
- **Task:** tag `Task` so it matches the wire format and make `Encode` send `[]` for a nil `Tags` without modifying the caller's value.
- **Acceptance:** `TestEncodeGolden`, `TestEncodeDoesNotMutateInput`, `TestDecode`, `TestRoundTrip`.
- <details><summary>Hint</summary>Pointers (`*string`, `*int`) give you `null`; `Encode` receives a copy of the struct, so fixing `Tags` locally is safe.</details>

### Exercise 2: Date and enum types
- **Task:** implement `Date` (`YYYY-MM-DD`) and `Status` (string on the wire, `int` in memory) so `Task` round-trips and bad input is rejected.
- **Acceptance:** `TestMarshalTask`, `TestUnmarshalTask`, `TestUnmarshalRejectsBadInput`, `TestStatusAsMapKey`, ...
- <details><summary>Hint</summary>Marshal a formatted string with `json.Marshal`; unmarshal into a `string` first and then `time.Parse("2006-01-02", s)`.</details>

### Exercise 3: Strict PATCH
- **Task:** implement `Optional[T].UnmarshalJSON`, a strict `Decode` and `Patch.Apply` that validates every present field and reports all errors at once.
- **Acceptance:** `TestDecodeTriState`, `TestDecodeRejects`, `TestApplyNullClears`, `TestApplyValidation`, `TestApplyDoesNotAliasPatchTags`, ...
- <details><summary>Hint</summary>`Set` is true whenever the method is called; compare the raw bytes with `"null"` for `Null`; build the new `Task` from a copy and return the original if `fields` is non-empty.</details>

## 7. Recap
- The wire format is a contract: tag every field, make `nil` slices `[]`, and keep dates and enums as strings.
- `omitempty` is about *zero values*, not absence; use pointers, `omitzero` or `Optional[T]` deliberately.
- Decode strictly (unknown fields, trailing data, size limit), then validate; map the two failures to 400 and 422.
- Reach for `encoding/json` + typed structs by default, `json.RawMessage` for polymorphic payloads, and `MarshalText` for string-like enums.

## 8. Self-check Questions
1. Why does `{"due_date":null}` decode into a `*string` the same way as `{}`, and how does `Optional[T]` tell them apart?
2. Which receiver type must `UnmarshalJSON` use, and what happens if you pick the other one?
3. A client sends `{"title":"x","tittle":"y"}`. What does the default decoder do and how do you change it?
4. Why is a decoded `id` of `9007199254740993` wrong when the target is `map[string]any`?
5. Which situations produce 400 and which 422 in 3.5?

## 9. Further Reading
- [encoding/json](https://pkg.go.dev/encoding/json) package documentation (struct tag rules, `omitzero`)
- [JSON and Go](https://go.dev/blog/json) (Go blog)
- [yaml.v3](https://pkg.go.dev/gopkg.in/yaml.v3) documentation

---
**Next:** [05 Database Access](../05-database-access/README.md)
