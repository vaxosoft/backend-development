# 04. Serialization in Rust

> Estimated time: 1.5-2h | Difficulty: Intermediate | Prerequisites: 01 (traits, `Option`/`Result`), 02-03 (axum handlers, error envelope)

## Learning Outcomes
After this module you can:
- [ ] Encode and decode JSON with `serde` derives, and rename fields/variants to match the wire format
- [ ] Model "absent", `null` and "value" differently (`Option`, `default`, `Option<Option<T>>`)
- [ ] Reject or ignore unknown fields on purpose, and read serde's error messages
- [ ] Write custom (de)serializers (`with`, `deserialize_with`, `try_from`) and handle dates with `chrono`
- [ ] Read and write CSV, TOML and form-encoded bodies, and validate decoded data

## 1. Concept Overview
Serialization turns in-memory values into bytes (JSON, CSV, ...) and back. Three things go wrong in practice: the **shape** is not what you expected (missing/extra/`null` fields), the **values** are wrong (a date that is not a date), and the **data is well-formed but invalid** (a 300-character title). Keep those three failures apart: decoding handles the first two, validation the third.

```
bytes --decode--> typed value --validate--> domain value --encode--> bytes
         400 bad syntax / 422 wrong shape      422 rule broken
```

## 2. How Rust Approaches It
- **`serde`** is the standard: `#[derive(Serialize, Deserialize)]` generates code at compile time, so there is no reflection and no runtime schema. The *format* crates (`serde_json`, `csv`, `toml`) plug into the same derives.
- Attributes replace struct tags: `rename_all`, `default`, `skip_serializing_if`, `deny_unknown_fields`, `alias`, `with`, `try_from`.
- The target type is the schema. `Option<T>` is "may be absent or null"; a plain `T` is required. There is no `null` you can forget to check.
- Date/time: `chrono` with its `serde` feature (`NaiveDate` = `YYYY-MM-DD`, `DateTime<Utc>` = RFC 3339).
- In axum: `Json<T>` and `Form<T>` extractors use these same derives; their *rejections* are plain text unless you map them to your error envelope (example 3.5).
- XML and YAML are deliberately skipped: there is no stable, maintained serde crate worth teaching (see `PROFILE.md`).

## 3. Guided Examples

### 3.1 JSON basics
- **Goal:** encode, decode, read error messages, use `Value` when the shape is unknown.
- **Run:** `cargo run -p m04-serialization --example json_basics`
- **Code:** [examples/json_basics.rs](examples/json_basics.rs)
```rust
#[derive(Debug, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
struct Profile { display_name: String, is_admin: bool, scores: Vec<u32>, nickname: Option<String> }
```
- **Walkthrough:** `to_string` / `to_string_pretty` encode; `from_str::<Point>` decodes and ignores the extra `"ignored"` key by default. Errors say *what* and *where* (`line 1 column 8`), and `err.classify()` tells syntax (`Syntax`/`Eof`) from shape (`Data`). `value["a"]["b"]` never panics: a missing key is `Value::Null`.
- **Expected output:**
  ```
  {"x":3,"y":-4}
  {"displayName":"Ada","isAdmin":true,"scores":[10,20],"nickname":null}
  {
    "displayName": "Ada",
    "isAdmin": true,
    "scores": [
      10,
      20
    ],
    "nickname": null
  }
  Point { x: 1, y: 2 }
  {"x": 1}                 -> missing field `y` at line 1 column 8 (Data)
  {"x": "one", "y": 2}     -> invalid type: string "one", expected i32 at line 1 column 11 (Data)
  {"x": 1, "y": 2          -> EOF while parsing an object at line 1 column 15 (Eof)
  {"x": 1.5, "y": 2}       -> invalid type: floating point `1.5`, expected i32 at line 1 column 9 (Data)
  "Ada"
  Some("b")
  null
  {"count":2,"items":[1,2],"ok":true}
  ```

### 3.2 Optional fields, defaults, unknown fields
- **Goal:** control what "missing" and "null" mean.
- **Run:** `cargo run -p m04-serialization --example optional_fields`
- **Code:** [examples/optional_fields.rs](examples/optional_fields.rs)
```rust
#[serde(default, deserialize_with = "double_option")]
due: Option<Option<String>>,   // None = absent, Some(None) = null, Some(Some(v)) = value
```
- **Walkthrough:** `default` fills missing fields; `skip_serializing_if` removes `"note"` from the output instead of writing `null`; `deny_unknown_fields` turns typos into errors. The last loop shows the PATCH trap: a plain `Option<Option<T>>` maps `null` to `None`, the same as "absent".
- **Expected output:**
  ```
  Settings { name: "api", retries: 0, timeout_secs: 30, note: None }
  {"name":"api","retries":0,"timeout_secs":30}
  {"name":"api","retries":0,"timeout_secs":30,"note":"hi"}
  None
  invalid type: null, expected a string at line 1 column 13
  a
  unknown field `extra`, expected `name` at line 1 column 21
  {}                       naive=None  double_option=None
  {"due": null}            naive=None  double_option=Some(None)
  {"due": "2030-01-31"}    naive=Some(Some("2030-01-31"))  double_option=Some(Some("2030-01-31"))
  ```

### 3.3 Dates, enums, custom (de)serializers, validation in the type
- **Goal:** `chrono` dates, `alias`, `with`, `deserialize_with`, `try_from`.
- **Run:** `cargo run -p m04-serialization --example custom_types`
- **Code:** [examples/custom_types.rs](examples/custom_types.rs)
```rust
#[derive(Debug, Serialize, Deserialize)]
#[serde(try_from = "String")]
struct Email(String);          // decoding runs TryFrom<String>: invalid input never becomes an Email
```
- **Walkthrough:** an offset timestamp (`+03:00`) is normalised to UTC; `ts_seconds` stores a Unix timestamp; `normalized_tags` trims and lowercases while decoding; `"doing"` is accepted as an alias of `in_progress` but always written as `in_progress`.
- **Expected output:**
  ```
  Item { status: InProgress, due: 2030-01-31, created_at: 2030-01-01T09:00:00Z, updated_at: 2030-01-01T00:00:00Z, tags: ["docs", "p1"] }
  {"status":"in_progress","due":"2030-01-31","created_at":"2030-01-01T09:00:00Z","updated_at":1893456000,"tags":["docs","p1"]}
  unknown variant `blocked`, expected one of `todo`, `doing`, `in_progress`, `done` at line 1 column 20
  input is out of range at line 1 column 38
  input contains invalid characters at line 1 column 66
  ada@example.com
  `ada` is not an email address at line 1 column 16
  ```

### 3.4 CSV and TOML
- **Goal:** the same derives drive other formats.
- **Run:** `cargo run -p m04-serialization --example csv_and_toml`
- **Code:** [examples/csv_and_toml.rs](examples/csv_and_toml.rs)
- **Walkthrough:** `csv::Writer::serialize` writes the header from field names and quotes cells with commas or quotes. `Option` is an empty cell. CSV is flat, so a `Vec` field needs a custom `with` module (exercise 3). Errors name the record and line. `toml` is the config format used in module 09.
- **Expected output:**
  ```
  id,title,due_date,done
  1,"Write docs, then review",2030-01-31,false
  2,"Say ""hi""",,true
  Row { id: 1, title: "Write docs, then review", due_date: Some(2030-01-31), done: false }
  Row { id: 2, title: "Say \"hi\"", due_date: None, done: true }
  error: CSV deserialize error: record 2 (line: 3, byte: 35): field 0: invalid digit found in string
  Config { name: "tracker", server: Server { host: "127.0.0.1", port: 8080 }, features: [] }
  name = "tracker"
  features = []

  [server]
  host = "127.0.0.1"
  port = 8080
  invalid type: integer `1`, expected a string
  ```

### 3.5 Task Tracker API: JSON and form bodies, error envelope
- **Goal:** decode requests, validate, and answer errors with the shared envelope.
- **Run:** `ADDR=127.0.0.1:8080 cargo run -p m04-serialization --example task_api` (Ctrl-C or SIGTERM stops it cleanly)
- **Code:** [examples/task_api.rs](examples/task_api.rs)
```rust
#[derive(FromRequest)]
#[from_request(via(Json), rejection(ApiError))]
struct ApiJson<T>(T);          // Json<T>, but rejections use our envelope
```
- **Walkthrough:** `JsonRejection::JsonDataError` (well-formed JSON, wrong shape) becomes **422**, syntax and content-type problems **400**. After decoding, `validate` collects *all* rule violations into `fields`. The form endpoint takes `title=...&tags=a,b` because `application/x-www-form-urlencoded` has no arrays; an empty `due_date=` is mapped to `None` by `empty_as_none`. `created_at` is a fixed clock so the output is reproducible.
- **Expected output** (headers trimmed to the interesting ones):
  ```
  $ curl -i -X POST localhost:8080/tasks -H 'content-type: application/json' \
      -d '{"title":"Write docs","status":"in_progress","due_date":"2030-01-31","tags":["docs","p1"]}'
  HTTP/1.1 201 Created
  location: /tasks/1

  {"id":1,"title":"Write docs","status":"in_progress","due_date":"2030-01-31","tags":["docs","p1"],"owner_id":null,"created_at":"2030-01-01T09:00:00Z"}

  $ curl -i -X POST localhost:8080/tasks -H 'content-type: application/json' -d '{"title":'
  HTTP/1.1 400 Bad Request
  {"error":{"code":"bad_request","message":"Failed to parse the request body as JSON: title: EOF while parsing a value at line 1 column 9"}}

  $ curl -i -X POST localhost:8080/tasks -H 'content-type: application/json' -d '{"title":"x","colour":"red"}'
  HTTP/1.1 422 Unprocessable Entity
  {"error":{"code":"validation_failed","fields":{"body":"Failed to deserialize the JSON body into the target type: colour: unknown field `colour`, expected one of `title`, `status`, `due_date`, `tags`, `owner_id` at line 1 column 21"},"message":"invalid task"}}

  $ curl -i -X POST localhost:8080/tasks -H 'content-type: application/json' -d '{"title":"  ","tags":["Docs","docs"]}'
  HTTP/1.1 422 Unprocessable Entity
  {"error":{"code":"validation_failed","fields":{"tags":"tags must be lowercase","title":"must be 1-200 characters"},"message":"invalid task"}}

  $ curl -i -X POST localhost:8080/tasks/form -d 'title=Buy+milk&status=done&due_date=&tags=home,%20errands'
  HTTP/1.1 201 Created
  location: /tasks/2

  {"id":2,"title":"Buy milk","status":"done","due_date":null,"tags":["home","errands"],"owner_id":null,"created_at":"2030-01-01T09:00:00Z"}

  $ curl -i localhost:8080/tasks/99
  HTTP/1.1 404 Not Found
  {"error":{"code":"not_found","message":"task not found"}}
  ```
  (`json!` orders object keys alphabetically, so `message` follows `fields`; JSON object order is not significant.)

## 4. Common Pitfalls

| Mistake | Why it happens | Fix |
|---------|----------------|-----|
| PATCH cannot clear a field (`null` is ignored) | `Option<Option<T>>` deserializes `null` as the outer `None` | `#[serde(default, deserialize_with = "double_option")]` (ex. 2) |
| Typos in client JSON are silently dropped | serde ignores unknown fields by default | `#[serde(deny_unknown_fields)]` on request types (not with `flatten`) |
| Every `Option` field serializes as `"x":null` | derive writes `None` as `null` | `skip_serializing_if = "Option::is_none"`, or keep `null` when the contract says so (the Task wire format does) |
| `Json` extractor errors come back as plain text | axum's default rejection body is text | wrap it (`#[derive(FromRequest)] #[from_request(via(Json), rejection(ApiError))]`) |
| `Form` fails on `due_date=` (empty field) | urlencoded gives `""`, not "absent", and `""` is not a date | `deserialize_with = "empty_as_none"` |
| `Form`/`csv` cannot hold a `Vec` | these formats are flat | split a delimited string (`a,b`) via `with`/`deserialize_with` |
| Untagged enum errors say only "data did not match any variant" | `#[serde(untagged)]` hides the per-variant reason | prefer internally tagged (`tag = "type"`) enums for APIs |
| Treating a successful decode as "valid" | decoding checks types, not business rules | validate after decoding (or `try_from`) and answer 422 with `fields` |
| Integers lose precision in JavaScript clients | `i64`/`u64` above 2^53 | send ids that fit, or serialize as strings with a `with` module |

## 5. Coming From Other Languages
- **Go (`encoding/json`):** struct tags become `#[serde(...)]` attributes, but a missing field is a *decode error* for a plain `T` (Go gives the zero value). `omitempty` ~ `skip_serializing_if`; `DisallowUnknownFields` ~ `deny_unknown_fields`; `*T`/`json.RawMessage` ~ `Option<T>`/`Value`.
- **TypeScript/Python:** no `any` unless you ask for `serde_json::Value`; the type is checked while decoding, no separate runtime validator (like zod/pydantic) is needed for shape.
- **Java/C# (Jackson, System.Text.Json):** derives replace annotations/reflection; `try_from` plays the role of a validating constructor or custom converter.

## 6. Exercises
All crates are in [exercises/](exercises/); reference solutions in [solutions/](solutions/).

| # | Title | Difficulty | Skill practiced | Run tests |
|---|-------|-----------|-----------------|-----------|
| 1 | Create-task JSON | Easy | serde attributes: `rename_all`, `default`, `deny_unknown_fields` | `cargo test -p m04-ex1-create-task-json` |
| 2 | PATCH semantics | Medium | custom `deserialize_with`, `Option<Option<T>>` | `cargo test -p m04-ex2-patch-semantics` |
| 3 | CSV import | Stretch | `with` module, `csv` reader/writer, validating decoded rows | `cargo test -p m04-ex3-csv-import` |

**Exercise 1 - Create-task JSON.** *Task:* the types in `src/lib.rs` use serde defaults; make them match the wire format (`in_progress`, only `title` required, unknown fields rejected). *Acceptance:* all tests in `tests/wire_format.rs`. *Hint:* three attributes are enough: one on the enum, one on `CreateTask`, and `#[serde(default)]` on the optional fields.

**Exercise 2 - PATCH semantics.** *Task:* implement `double_option` and `Task::apply` so absent leaves a field alone, `null` clears `due_date`, a value sets it. *Acceptance:* `tests/patch.rs`. *Hint:* `Option::<T>::deserialize(d)` then wrap the result in `Some`.

**Exercise 3 - CSV import.** *Task:* write `tags_csv::{serialize, deserialize}` (`docs;p1`), `to_csv`, and `from_csv`, which also validates title and tags and reports the row and field. *Acceptance:* `tests/csv_import.rs`. *Hint:* `csv::Writer::from_writer(Vec::new())`, `writer.into_inner()`; number rows with `enumerate()` while iterating `reader.deserialize::<TaskRow>()`.

## 7. Recap
- The type is the schema: `Option` for optional, `#[serde(default)]` for defaults, attributes for names and strictness.
- Decode errors (400/422) and validation errors (422 + `fields`) are different failures; keep both visible to clients.
- `null` vs absent needs `Option<Option<T>>` plus a custom deserializer.
- Use `serde_json::Value` only at the edges where the shape is truly dynamic.
- Reach for plain derives first, then `with`/`try_from` for the few fields that need custom rules.

## 8. Self-check Questions
1. A client sends `{"title":"x","tittle":"y"}`. What happens with and without `deny_unknown_fields`, and which status would you return?
2. Why does `Option<Option<NaiveDate>>` alone fail to represent `"due_date": null` in a PATCH body?
3. Where would you put the rule "tags must be lowercase": in the `Deserialize` impl, a `try_from` newtype, or a `validate` function? What changes for the client's error response?
4. Why can't a `Vec<String>` field go into a CSV cell, and what are two ways to cope?

## 9. Further Reading
- serde: <https://serde.rs> (attributes reference: <https://serde.rs/attributes.html>)
- `serde_json`: <https://docs.rs/serde_json>, `csv`: <https://docs.rs/csv>, `toml`: <https://docs.rs/toml>
- axum extractors and rejections: <https://docs.rs/axum/latest/axum/extract/index.html>

---
**Next:** [05 Database Access](../05-database-access/README.md)
