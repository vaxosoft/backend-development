# 04. Serialization in TypeScript

> Estimated time: 1.5-2h | Difficulty: Intermediate | Prerequisites: 01 (language fundamentals), 03 (REST API design)

## Learning Outcomes

After this module you can:

- [ ] Encode and decode JSON with `JSON.stringify` / `JSON.parse` and explain what is lost on the way (`undefined`, `Date`, `bigint`, `Map`, `NaN`, numbers above 2^53)
- [ ] Validate untrusted decoded data with zod and derive the static type from the same schema
- [ ] Model optional vs nullable fields, unknown keys, defaults and PATCH semantics (absent vs `null`)
- [ ] Handle dates as RFC 3339 / `YYYY-MM-DD` strings on the wire
- [ ] Read and write CSV, YAML and form-encoded data, and turn validation failures into the shared error envelope

## 1. Concept Overview

Serialization turns in-memory values into bytes for the wire (encode) and back again (decode). A service has two boundaries: **outgoing** data you control, and **incoming** data you do not. Decoding must therefore always be followed by **validation**: JSON that parses is not JSON that is correct.

```
bytes --parse--> unknown --validate/transform--> typed value --business logic--> typed value --encode--> bytes
```

Decide early which failures are which: malformed syntax is a `400 bad_request`, well-formed JSON with wrong content is a `422 validation_failed` with per-field messages (see [domain-and-projects.md](../../../../framework/domain-and-projects.md)). The wire format of the Task Tracker uses `snake_case` keys, `YYYY-MM-DD` dates and RFC 3339 UTC timestamps in every language.

## 2. How TypeScript Approaches It

- **`JSON.stringify` / `JSON.parse`** are built in and fast. `parse` returns `any`, so the compiler checks nothing about the result. Types are erased at runtime.
- **zod 4** is the runtime schema library chosen in [PROFILE.md](../../PROFILE.md): one schema validates, transforms and (with `z.infer`) yields the static type. Use `safeParse` in handlers (a discriminated union) and `parse` where a failure is a bug.
- **`replacer` / `reviver` / `toJSON`** customise encoding; prefer an explicit mapping function (`toWire(task)`) over clever global hooks.
- **`csv-parse` / `csv-stringify`** (sync API used here) for CSV, **`yaml`** for YAML, **`URLSearchParams`** for form bodies. XML is skipped: there is no stable built-in and it is rare in new services.
- Non-idiomatic: `JSON.parse(text) as Task` (a lie to the compiler), and classes with decorators for serialization.

| Need                 | Tool                                                                         |
| -------------------- | ---------------------------------------------------------------------------- |
| Encode a custom type | `toJSON()` on the class, a `replacer`, or a mapper function                  |
| Decode + validate    | `Schema.safeParse(JSON.parse(text))`                                         |
| Absent vs `null`     | `.optional()` vs `.nullable()`; `'key' in obj` in PATCH                      |
| Reject unknown keys  | `z.strictObject({...})` (default `z.object` silently strips them)            |
| String to `Date`     | `.transform((s) => new Date(s))`, or keep the string and convert at the edge |

## 3. Guided Examples

### 3.1 What JSON can and cannot carry

- **Goal:** know the `JSON.stringify` / `JSON.parse` quirks before they bite.
- **Run:** `node modules/04-serialization/examples/01_json_quirks.ts`
- **Code:** [01_json_quirks.ts](examples/01_json_quirks.ts)
- **Walkthrough:** `undefined`, functions and symbols vanish from objects (and become `null` in arrays); `Set` and `Map` become `{}`; `NaN` becomes `null`; `bigint` throws; `9007199254740993` is parsed as `...992`. A `replacer` maps `bigint` and `Set`, `toJSON()` lets a class pick its own encoding, a `reviver` rebuilds `Date`s. Assigning `JSON.parse(...)` to a typed variable checks nothing.
- **Expected output:**

  ```
  {"id":1,"title":"Write docs","due":"2030-01-31T00:00:00.000Z","tags":{},"meta":{},"score":null}
  [null,null,null,null]
  TypeError: Do not know how to serialize a BigInt
  9007199254740992
  {"big":"9223372036854775808","set":[1,2],"price":"12.50 EUR"}
  true 9
  number
  SyntaxError
  ```

### 3.2 Decoding untrusted JSON with zod

- **Goal:** one schema for validation and the static type; optional vs nullable; unknown keys.
- **Run:** `node modules/04-serialization/examples/02_zod_decoding.ts`
- **Code:** [02_zod_decoding.ts](examples/02_zod_decoding.ts)

  ```ts
  const CreateUser = z.strictObject({
    name: z.string().trim().min(1).max(100),
    nickname: z.string().optional(),                      // key may be absent
    manager_id: z.number().int().nullable().default(null), // key may be null
  });
  type CreateUser = z.infer<typeof CreateUser>;
  ```

- **Walkthrough:** `safeParse` never throws; `result.error.issues` lists every problem with a `path`. `strictObject` turns an extra `admin` key into an error. A `transform` changes the output type, so `z.input` and `z.output` differ.
- **Expected output:**

  ```
  valid: ok {"name":"Ada","email":"ada@example.com","manager_id":null}
  null manager: ok {"name":"Bob","email":"b@x.io","manager_id":null}
  wrong types: name -> Invalid input: expected string, received number
  wrong types: email -> Invalid string: must include "@"
  wrong types: manager_id -> Invalid input: expected int, received number
  unknown key: (root) -> Unrecognized key: "admin"
  not an object: (root) -> Invalid input: expected object, received string
  null undefined
  ZodError issues: 2
  2030-01-01T09:00:00.000Z
  2030-06-01T00:00:00Z
  ```

### 3.3 CSV, YAML and form-encoded data

- **Goal:** the other formats, briefly.
- **Run:** `node modules/04-serialization/examples/03_csv_yaml_form.ts`
- **Code:** [03_csv_yaml_form.ts](examples/03_csv_yaml_form.ts)
- **Walkthrough:** CSV cells are always strings (quoting is automatic); `cast` can convert columns, but remember the header row also passes through it. YAML 1.2 keeps `2030-01-31` a string. `URLSearchParams` returns `null` for a missing key, `''` for an empty one, and `Object.fromEntries` keeps only the last value of a repeated key.
- **Expected output:**

  ```
  id,title,tags
  1,"Write docs, then review",docs|p1
  2,"Say ""hello""",
  Write docs, then review string
  [ { id: 1, done: true }, { id: 2, done: false } ]
  {
    server: { host: '127.0.0.1', port: 8080, tags: [ 'a', 'b' ] },
    due: '2030-01-31'
  }
  title: Write docs
  tags:
    - docs
    - p1
  due_date: null
  Buy milk [ 'home', 'p1' ] "" null
  { title: 'Buy milk', tag: 'p1', empty: '' }
  q=a%26b%3Dc&page=2
  ```

### 3.4 The Task Tracker wire format

- **Goal:** create and PATCH schemas for the shared domain, and the `422` error envelope.
- **Run:** `node modules/04-serialization/examples/04_task_wire_format.ts`
- **Code:** [04_task_wire_format.ts](examples/04_task_wire_format.ts)

  ```ts
  const PatchTask = z
    .strictObject({ title: Title, status: Status, due_date: DueDate.nullable(), tags: Tags })
    .partial(); // absent = unchanged, due_date: null = clear
  ```

- **Walkthrough:** `toEnvelope` collapses zod issues into `fields` (first message per path, unknown keys named). `rfc3339()` drops the milliseconds so `created_at` matches the shared example. `Object.keys(patch.data)` shows that absent keys stay absent while `due_date: null` survives validation.
- **Expected output:**

  ```
  {"id":1,"title":"Write docs","status":"todo","due_date":null,"tags":["docs","p1"],"owner_id":null,"created_at":"2030-01-01T09:00:00Z"}
  {"error":{"code":"validation_failed","message":"invalid task","fields":{"title":"must be 1-200 characters","due_date":"must be a YYYY-MM-DD date","tags.0":"tags must be lowercase","tags":"tags must be unique","x":"unknown field"}}}
  {"title":"New"} -> keys: [ 'title' ] clears due: false
  {"due_date":null} -> keys: [ 'due_date' ] clears due: true
  {"status":"x"} -> {"error":{"code":"validation_failed","message":"invalid task","fields":{"status":"Invalid option: expected one of \"todo\"|\"in_progress\"|\"done\""}}}
  400 bad_request
  ```

## 4. Common Pitfalls

| Mistake                                                                     | Why it happens                                                              | Fix                                                                                                   |
| --------------------------------------------------------------------------- | --------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------- |
| `JSON.parse(body) as Task`                                                  | `parse` returns `any`; the cast silences the compiler but validates nothing | `Schema.safeParse(JSON.parse(body))`, use the parsed `data`                                           |
| `JSON.stringify` throws on `bigint`                                         | JSON has no 64-bit integer type                                             | send ids as strings, or add a `replacer`; never `Number(big)` for ids above 2^53                      |
| Large ids silently change (`...993` becomes `...992`)                       | all JSON numbers are doubles in JavaScript                                  | transmit such ids as strings                                                                          |
| A field disappears after a round trip                                       | `undefined`, functions and symbols are dropped; `Map`/`Set` become `{}`     | convert explicitly (`[...set]`, `Object.fromEntries(map)`) before encoding                            |
| `z.object` accepts `{admin: true}`                                          | non-strict objects strip unknown keys without an error                      | `z.strictObject` for request bodies you want to reject                                                |
| PATCH cannot clear a field                                                  | code checks `if (patch.due_date)`, or the schema rejects `null`             | `.nullable()` in the schema and test `'due_date' in patch` (or `!== undefined`) instead of truthiness |
| `new Date('2030-01-31')` is UTC but `new Date('2030-01-31T00:00')` is local | the two ISO-like forms parse differently                                    | keep `YYYY-MM-DD` as a string for dates; use `toISOString()` for instants                             |
| Default values are lost on the type side                                    | `z.infer` is the output type; callers send the input type                   | use `z.input<typeof S>` for what clients may send                                                     |

## 5. Coming From Other Languages

- **JavaScript:** the same `JSON` object, but TypeScript's types give a false sense of safety after `parse`; zod is the runtime half.
- **Java / C#:** no annotations or reflection-driven mappers (Jackson, `System.Text.Json`); the schema is an ordinary value and the type is inferred from it. `z.object` strips unknown keys silently (Jackson fails on them by default), so opt in to `strictObject`.
- **Python:** zod plays the role of pydantic: validate and coerce at the boundary, not inside business logic.
- **Go:** no struct tags; field renaming is an explicit mapping because the object keys already are the wire names (`snake_case` in the schema).

## 6. Exercises

Starter code and tests are in [exercises/](exercises/); reference solutions in [solutions/](solutions/). Copy a solution over the exercise to compare.

| #   | Title                                                           | Difficulty | Skill practiced                                             | Run tests                                                                   |
| --- | --------------------------------------------------------------- | ---------- | ----------------------------------------------------------- | --------------------------------------------------------------------------- |
| 1   | [JSON codec](exercises/ex1_json_codec/json-codec.ts)            | Easy       | `replacer`, `reviver`, non-throwing decode                  | `node --test "modules/04-serialization/exercises/ex1_json_codec/*.test.ts"` |
| 2   | [Task input validation](exercises/ex2_task_input/task-input.ts) | Medium     | zod schemas, defaults, PATCH absent vs `null`, field errors | `node --test "modules/04-serialization/exercises/ex2_task_input/*.test.ts"` |
| 3   | [CSV import/export](exercises/ex3_csv_tasks/csv-tasks.ts)       | Stretch    | CSV round trip, per-row validation with row numbers         | `node --test "modules/04-serialization/exercises/ex3_csv_tasks/*.test.ts"`  |

### Exercise 1: JSON codec

- **Task:** implement `encodeJson` (bigint to string, `Set` to array, `Map` to object), `decodeJson` (never throws) and `decodeWithDates` (`*_at` strings become `Date`).
- **Acceptance:** all tests in `json-codec.test.ts` pass.
- **Hint:** a replacer sees the value after `toJSON`, and is called for nested values too.

### Exercise 2: Task input validation

- **Task:** implement `parseCreateTask` and `parsePatchTask` following the rules in the doc comments; failures return `{ ok: false, fields }`.
- **Acceptance:** all tests in `task-input.test.ts` pass, including defaults, strict keys and `due_date: null` in a patch.
- **Hint:** build one `z.strictObject` for create and derive the patch with `.partial()` of the patchable fields; map `issue.path[0]` to `fields`, and read `issue.keys` for `unrecognized_keys`.

### Exercise 3: CSV import/export

- **Task:** implement `tasksToCsv` and `tasksFromCsv`. Valid rows are returned, invalid rows are reported as `{ row, message }`; a missing header column throws.
- **Acceptance:** all tests in `csv-tasks.test.ts` pass, including a title with commas, quotes and a newline.
- **Hint:** the `columns` option of `csv-parse` can be a function that receives the header; validate each record with a zod schema whose `id` is a regex-checked string transformed to a number.

## 7. Recap

- Decode to `unknown`, validate, then use typed data. Never trust `JSON.parse` output or a type assertion.
- JSON loses `undefined`, `Date`, `bigint`, `Map`, `Set`; encode them explicitly. Large integers travel as strings.
- Distinguish absent from `null` (PATCH), and malformed (400) from invalid (422).
- Dates on the wire are strings: `YYYY-MM-DD` for dates, RFC 3339 UTC for instants.
- Reach for zod at every boundary, `toJSON`/mapper functions for outgoing data, and a CSV/YAML library only when the format is requested.

## 8. Self-check Questions

1. Why is `const t: Task = JSON.parse(text)` accepted by the compiler even when `text` is `"42"`?
2. What is the difference between `z.object` and `z.strictObject`, and when do you want each?
3. A client sends `{"due_date": null}` in a PATCH. How do you tell this apart from `{}`, and what should each do?
4. Why does `JSON.stringify({ id: 2n ** 63n })` throw, and what are two safe ways to transmit such an id?
5. Which HTTP status do you return for `{"title":` and which for `{"title":""}`, and why?

## 9. Further Reading

- MDN: [JSON](https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Global_Objects/JSON) and [`JSON.stringify` replacer](https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Global_Objects/JSON/stringify)
- [zod documentation](https://zod.dev) (basic usage, objects, error formatting)
- [csv-parse](https://csv.js.org/parse/) and [yaml](https://eemeli.org/yaml/) documentation

---

**Next:** [05 Database Access](../05-database-access/README.md)
