# 04. Serialization in Python

> Estimated time: 1.5-2h | Difficulty: Intermediate | Prerequisites: 01 Language Fundamentals, 03 REST API Design

## Learning Outcomes
After this module you can:
- [ ] Encode and decode JSON with `json`, and name the types it cannot handle (`datetime`, `Decimal`, `set`).
- [ ] Declare a Pydantic model that validates, decodes and encodes the Task Tracker wire format.
- [ ] Tell a missing field from an explicit `null` (PATCH semantics) and reject unknown fields.
- [ ] Format timestamps as RFC 3339 UTC and refuse naive datetimes.
- [ ] Read and write CSV, XML, TOML, YAML (safely) and form-encoded data.
- [ ] Turn a validation failure into the shared `fields` error envelope.

## 1. Concept Overview
Serialization turns in-memory objects into bytes for the wire, a file or a database, and back again. Everything that crosses a trust boundary must be **validated on the way in**: decoding gives you data of the right *shape*, validation makes sure it has the right *meaning* (lengths, enums, ranges).

```
bytes --decode--> generic data (dict/list) --validate--> typed model --encode--> bytes
```

Three questions come up in every language: how are absent, `null` and empty values told apart, what happens to unknown fields, and how are times and decimals written. The Task Tracker wire format answers them once: `snake_case` keys, `YYYY-MM-DD` dates, RFC 3339 UTC timestamps, and unknown fields in a request are an error.

## 2. How Python Approaches It

| Need | Tool | Notes |
|------|------|-------|
| JSON | `json` (stdlib) | `dumps`/`loads`; only `dict`, `list`, `str`, `int`, `float`, `bool`, `None`. Everything else needs `default=` |
| Typed JSON + validation | `pydantic` v2 | `model_validate_json`, `model_validate`, `model_dump(mode="json")`, `model_dump_json` |
| CSV | `csv` (stdlib) | open files with `newline=""`; all values are `str` |
| XML | `xml.etree.ElementTree` | build trees instead of concatenating strings |
| TOML | `tomllib` (stdlib) | read-only |
| YAML | `PyYAML` | **always** `yaml.safe_load` |
| Forms / query strings | `urllib.parse` (`urlencode`, `parse_qs`) | values are lists |

Idiomatic Python declares the shape once as a type-annotated class and lets the library do the work. The non-idiomatic style is hand-written `if "title" not in data` checks on raw dicts, or `json.dumps(obj.__dict__)`. Pydantic is the one third-party choice here (it is what FastAPI uses, see [module 03](../03-rest-api-design/README.md)): validation is compiled from your type hints and errors come with a precise location.

## 3. Guided Examples

### 3.1 `json` basics and its limits
- **Goal:** know what round-trips and what raises.
- **Run:** `python modules/04-serialization/examples/01_json_basics.py`
- **Code:** [examples/01_json_basics.py](examples/01_json_basics.py)
- **Walkthrough:** tuples come back as lists and integer keys as strings; `datetime`, `Decimal` and `set` raise `TypeError` unless you supply `default=`; `allow_nan=False` keeps invalid JSON from leaving your service.

```python
def encode_extra(value: object) -> object:
    if isinstance(value, datetime):
        return value.isoformat().replace("+00:00", "Z")
    if isinstance(value, set):
        return sorted(value)
    raise TypeError(f"cannot serialise {type(value).__name__}")
```

- **Expected output:**

  ```
  {"id": 1, "title": "Write docs", "tags": ["docs", "p1"], "owner_id": null, "done": false}
  True
  {"done":false,"id":1,"owner_id":null,"tags":["docs","p1"],"title":"Write docs"}
  {'point': [1, 2], '7': 'int key'}
  TypeError: Object of type datetime is not JSON serializable
  TypeError: Object of type Decimal is not JSON serializable
  TypeError: Object of type set is not JSON serializable
  {"created_at": "2030-01-01T09:00:00Z", "tags": ["a", "b"]}
  JSONDecodeError: Illegal trailing comma before end of object at column 14
  nan
  ValueError: Out of range float values are not JSON compliant: nan
  ```

### 3.2 Pydantic models
- **Goal:** decode, validate and encode with one declaration; see missing vs `null`.
- **Run:** `python modules/04-serialization/examples/02_pydantic_models.py`
- **Code:** [examples/02_pydantic_models.py](examples/02_pydantic_models.py)
- **Walkthrough:** `model_dump()` keeps Python objects (`date`, enum) while `model_dump(mode="json")` produces JSON-safe values. `extra="forbid"` turns unknown keys into errors, `AwareDatetime` rejects naive timestamps, and all problems are reported together. `model_fields_set` / `exclude_unset=True` reveal what the client actually sent.

```python
class TaskUpdate(BaseModel):
    title: str | None = None
    due_date: date | None = None


update = TaskUpdate.model_validate_json('{"due_date": null}')
update.model_fields_set  # {'due_date'}: sent as null, so "clear it"
```

- **Expected output:**

  ```
  Task(id=1, title='Write docs', status=<Status.TODO: 'todo'>, due_date=datetime.date(2030, 1, 31), tags=[], created_at=datetime.datetime(2030, 1, 1, 9, 0, tzinfo=TzInfo(0)))
  {'id': 1, 'title': 'Write docs', 'status': <Status.TODO: 'todo'>, 'due_date': datetime.date(2030, 1, 31), 'tags': [], 'created_at': datetime.datetime(2030, 1, 1, 9, 0, tzinfo=TzInfo(0))}
  {'id': 1, 'title': 'Write docs', 'status': 'todo', 'due_date': '2030-01-31', 'tags': [], 'created_at': '2030-01-01T09:00:00Z'}
  {"id":1,"title":"Write docs","status":"todo","due_date":"2030-01-31","tags":[],"created_at":"2030-01-01T09:00:00Z"}
  7
  ('status',) enum - Input should be 'todo', 'in_progress' or 'done'
  ('colour',) extra_forbidden - Extra inputs are not permitted
  ('created_at',) timezone_aware - Input should have timezone info
  ('id',) missing - Field required
  ('created_at',) missing - Field required
  {"title": "New"} -> ['title'] {'title': 'New'}
  {"due_date": null} -> ['due_date'] {'due_date': None}
  2030-01-01T10:00:00+01:00 -> 2030-01-01T09:00:00+00:00
  ```

### 3.3 Other formats
- **Goal:** CSV, XML, TOML, YAML and form-encoded data with the right tool each.
- **Run:** `python modules/04-serialization/examples/03_other_formats.py`
- **Code:** [examples/03_other_formats.py](examples/03_other_formats.py)
- **Walkthrough:** the `csv` module quotes the embedded comma and quotes for you; `ElementTree` escapes `&` and `<`; `tomllib` returns typed values; `yaml.safe_load` parses plain data (note the unquoted `no` becoming `False`) and refuses Python object tags; `parse_qs` returns lists because keys may repeat.
- **Expected output:**

  ```
  id,title,tags
  1,"Say ""hi"", then leave",docs;p1
  2,Plain,
  {'id': '1', 'title': 'Say "hi", then leave', 'tags': 'docs;p1'}
  {'id': '2', 'title': 'Plain', 'tags': ''}
  <task id="1"><title>Fish &amp; chips &lt;urgent&gt;</title><tags><tag>food</tag><tag>p1</tag></tags></task>
  1 Fish & chips <urgent> ['food', 'p1']
  {'title': 'Task Tracker', 'port': 8080, 'debug': False, 'db': {'url': 'tasks.db'}}
  {'title': 'Write docs', 'tags': ['docs', 'p1'], 'due_date': datetime.date(2030, 1, 31), 'enabled': False}
  safe_load refused: ConstructorError
  title=Buy+milk+%26+eggs&tag=food&tag=p1
  {'title': ['Buy milk & eggs'], 'tag': ['food', 'p1']}
  ```

### 3.4 Custom validators and serializers
- **Goal:** normalise on the way in, control the format on the way out, rename keys.
- **Run:** `python modules/04-serialization/examples/04_custom_serializers.py`
- **Code:** [examples/04_custom_serializers.py](examples/04_custom_serializers.py)
- **Walkthrough:** `Annotated[str, BeforeValidator(...), StringConstraints(...)]` makes a reusable `Tag` type. `@field_validator` runs after type coercion, `@field_serializer` and `PlainSerializer` control the output (here: `Decimal` as a string, timestamps in UTC with a `Z`). `alias_generator=to_camel` accepts and emits camelCase while the code stays snake_case.
- **Expected output:**

  ```
  'Ship it' ['docs', 'p1'] Decimal('1.50')
  {"title":"Ship it","tags":["docs","p1"],"estimateHours":"1.50","createdAt":"2030-01-01T09:30:15Z"}
  {"title":"Ship it","tags":["docs","p1"],"estimate_hours":"1.50","created_at":"2030-01-01T09:30:15Z"}
  ('title',) - Value error, must be 1-200 characters
  ('tags',) - Value error, must not contain duplicates
  ```

### 3.5 Task Tracker wire format
- **Goal:** the shared request/response/error shapes end to end (no server needed).
- **Run:** `python modules/04-serialization/examples/05_task_wire_format.py`
- **Code:** [examples/05_task_wire_format.py](examples/05_task_wire_format.py)
- **Walkthrough:** `TaskCreate` is what clients may send (`extra="forbid"`, so `id` is rejected); `Task` is what the server returns. `envelope()` flattens `ValidationError.errors()` into the `fields` map. Malformed JSON also surfaces as a `ValidationError` (type `json_invalid`), which maps to `400 bad_request` instead of `422`.
- **Expected output:**

  ```
  201 {"id": 1, "title": "Write docs", "status": "todo", "due_date": "2030-01-31", "tags": ["docs", "p1"], "owner_id": null, "created_at": "2030-01-01T09:00:00Z"}
  422 {"error": {"code": "validation_failed", "message": "invalid task", "fields": {"title": "must be 1-200 characters", "status": "Input should be 'todo', 'in_progress' or 'done'", "tags": "each tag must be 1-30 lowercase characters", "id": "Extra inputs are not permitted"}}}
  400 {"error": {"code": "bad_request", "message": "malformed JSON"}}
  {"items": [{"id": 1, "title": "Item 1", "status": "todo", "due_date": null, "tags": [], "owner_id": null, "created_at": "2030-01-01T09:00:00Z"}, {"id": 2, "title": "Item 2", "status": "todo", "due_date": null, "tags": [], "owner_id": null, "created_at": "2030-01-01T09:00:00Z"}], "total": 2, "limit": 20, "offset": 0}
  ```

## 4. Common Pitfalls

| Mistake | Why it happens | Fix |
|---------|----------------|-----|
| `TypeError: Object of type datetime is not JSON serializable` | `json` only knows the basic types (also `Decimal`, `set`, `UUID`, `date`) | pass `default=`, or dump through a Pydantic model with `mode="json"` |
| Naive datetimes on the wire (`2030-01-01T09:00:00`) | `datetime.now()` and `utcnow()` return naive values; `json` does not care | use `datetime.now(UTC)`; declare `AwareDatetime` in models |
| `yaml.load(text)` executing code or building arbitrary objects | the unsafe loader constructs Python objects from tags | `yaml.safe_load`; remember YAML 1.1 turns `no`, `off`, `on` into booleans, so quote strings |
| Unknown fields silently ignored | Pydantic's default is `extra="ignore"`, so typos like `"duedate"` disappear | `ConfigDict(extra="forbid")` on request models |
| Cannot tell "not sent" from `null` in PATCH | `None` is also the default value | `model_fields_set` or `model_dump(exclude_unset=True)` |
| `model_dump()` fed to `json.dumps` fails | the default mode keeps `date`, `Enum`, `Decimal` objects | `model_dump(mode="json")` or `model_dump_json()` |
| CSV rows with blank lines or broken quotes | file opened without `newline=""`, or rows built with `",".join` | `csv.writer`/`DictWriter` and `open(..., newline="")` |
| `"7"` accepted as the integer `7` | Pydantic's lax mode coerces compatible types | `ConfigDict(strict=True)` or `StrictInt` where coercion is unwanted |
| `json.loads("NaN")` succeeds | Python's JSON dialect accepts `NaN` and `Infinity` | encode with `allow_nan=False`; validate floats with `allow_inf_nan=False` |

## 5. Coming From Other Languages
- **Go:** struct tags (`json:"due_date,omitempty"`) become model fields plus `exclude_unset` or `exclude_none`; `dict[str, Any]` is the generic form (like `map[string]any`). Both ignore unknown fields by default; `DisallowUnknownFields` corresponds to `extra="forbid"`.
- **Java/C#:** Jackson / `System.Text.Json` annotations map to `Field(alias=...)`, `field_validator` and `ConfigDict`; there is no separate DTO mapper step because the model validates itself.
- **JavaScript/TypeScript:** `JSON.stringify` silently turns a `Date` into a string and drops `undefined`; Python raises instead. Pydantic plays the role `zod` plays in TypeScript.
- **Ruby:** `to_json` on arbitrary objects has no equivalent: Python never serialises objects implicitly.

## 6. Exercises
Each exercise has starter code and tests in [exercises/](exercises/). Solutions are in [solutions/](solutions/). Run all commands from `languages/python/`.

| # | Title | Difficulty | Skill practiced | Run tests |
|---|-------|-----------|-----------------|-----------|
| 1 | JSON encoder | Easy | `json.dumps(default=...)`, UTC timestamps | `pytest modules/04-serialization/exercises/ex1_json_encoder` |
| 2 | CSV export and import | Medium | `csv` quoting, `newline=""`, typed conversion | `pytest modules/04-serialization/exercises/ex2_csv_tasks` |
| 3 | PATCH semantics | Stretch | Pydantic validation, missing vs `null`, error mapping | `pytest modules/04-serialization/exercises/ex3_task_patch` |

### Exercise 1: JSON encoder
- **Task:** implement `encode_json` in [m04_json_encoder.py](exercises/ex1_json_encoder/m04_json_encoder.py): compact sorted output, aware datetimes as UTC `Z`, dates, decimals and sets.
- **Acceptance:** [test_m04_json_encoder.py](exercises/ex1_json_encoder/test_m04_json_encoder.py) passes (naive datetimes, NaN and unknown types are rejected).
- **Hint:** `default=` is only called for unknown objects, and `datetime` is a subclass of `date`: check it first.

### Exercise 2: CSV export and import
- **Task:** implement `tasks_to_csv` and `tasks_from_csv` in [m04_csv_tasks.py](exercises/ex2_csv_tasks/m04_csv_tasks.py).
- **Acceptance:** [test_m04_csv_tasks.py](exercises/ex2_csv_tasks/test_m04_csv_tasks.py) passes: the round trip survives commas, quotes and newlines in titles; missing columns and bad values raise `ValueError`.
- **Hint:** `csv.writer` and `csv.DictReader` work on any file-like object, including `io.StringIO(newline="")`.

### Exercise 3: PATCH semantics
- **Task:** declare `TaskPatch` and implement `apply_patch` in [m04_task_patch.py](exercises/ex3_task_patch/m04_task_patch.py).
- **Acceptance:** [test_m04_task_patch.py](exercises/ex3_task_patch/test_m04_task_patch.py) passes: absent keys stay, `null` clears only nullable fields, all problems are reported in one `PatchError`, the input dict is not mutated.
- **Hint:** make title/status/tags non-nullable types (so `null` is invalid) and apply only `model_fields_set`.

## 7. Recap
- `json` handles basic types only; everything else is your `default=` or a Pydantic model's `mode="json"`.
- Validate at the boundary: forbid unknown fields, require aware datetimes, report all errors at once.
- Missing, `null` and empty are three states: `model_fields_set` tells them apart.
- Use the stdlib for CSV, XML, TOML and forms; use `yaml.safe_load` for YAML.
- Reach for **plain `json`** for opaque payloads, for **Pydantic** whenever the shape matters.

## 8. Self-check Questions
1. Why does `json.dumps({"when": datetime.now()})` fail, and why is making it work with `str()` a bad idea for APIs?
2. A client sends `{"owner_id": null}` in a PATCH. How does your code know whether to clear the owner or ignore the key?
3. What is the difference between `model_dump()` and `model_dump(mode="json")`, and which one belongs in a database call versus an HTTP response?
4. Why is `yaml.load` dangerous while `yaml.safe_load` is not?

## 9. Further Reading
- [json module](https://docs.python.org/3/library/json.html) and [csv module](https://docs.python.org/3/library/csv.html)
- [Pydantic: models, validators and serialization](https://docs.pydantic.dev/latest/concepts/models/)
- [PyYAML documentation](https://pyyaml.org/wiki/PyYAMLDocumentation)

---
**Next:** [05 Database Access](../05-database-access/README.md)
