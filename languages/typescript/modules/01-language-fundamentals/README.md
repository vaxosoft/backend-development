# 01. Language Fundamentals in TypeScript

> Estimated time: 1.5-2 h | Difficulty: Beginner | Prerequisites: basic programming; Node.js 24+ installed (see [PROFILE.md](../../PROFILE.md))

## Learning Outcomes

After this module you can:

- [ ] Model data with interfaces, unions and generics, and narrow `unknown` input to a safe type
- [ ] Explain value vs reference semantics and avoid mutating shared objects by accident
- [ ] Use `Map`, `Set`, array methods, iterators, generators and closures
- [ ] Handle errors with `Error` subclasses (with `cause`), result unions and `async`/`await`
- [ ] Organise code in ES modules and run, type-check, format and lint it with the repo tooling

## 1. Concept Overview

Every backend language answers the same questions: how are data shapes described, how are failures reported, how is memory managed, and how is code split into modules. A server spends its life turning untrusted bytes into well-typed values and back, so the type system and the error model decide how many bugs reach production.

TypeScript adds a static type layer to JavaScript. The layer exists only at compile time: Node strips the annotations and runs plain JavaScript, so the compiler guarantees consistency of _your_ code but knows nothing about the data that arrives at runtime.

## 2. How TypeScript Approaches It

| Topic            | TypeScript / JavaScript idiom                                                                         |
| ---------------- | ----------------------------------------------------------------------------------------------------- |
| Types            | structural typing: an object fits a type if it has the right members, no `implements` needed          |
| Variants         | string-literal unions and _discriminated unions_ (`{ kind: 'a' } \| { kind: 'b' }`) instead of `enum` |
| Untrusted values | `unknown` + narrowing (`typeof`, `instanceof`, `in`, type guards); never `any`                        |
| Memory           | garbage collected; objects, arrays, `Map` and `Set` are references, primitives are copied             |
| Errors           | `throw` an `Error` subclass for exceptional failures, return `Result`-like unions for expected ones   |
| Concurrency      | one thread, an event loop, `Promise` and `async`/`await`                                              |
| Modules          | ES modules; imports are relative **with the `.ts` extension**, built-ins use `node:`                  |
| Tooling          | `node file.ts` (type stripping), `tsc` only type-checks, Biome formats and lints                      |

Non-idiomatic: Java-style class hierarchies for everything, `enum`/`namespace` (not erasable, so not allowed here), `any`, non-null `!`, `==`.

Commands (from `languages/typescript/`):

```bash
node modules/01-language-fundamentals/examples/01_types_and_narrowing.ts
npm run typecheck   # tsc -p .
npm run lint        # biome check .
```

## 3. Guided Examples

### 3.1 Types and narrowing

- **Goal:** structural typing, discriminated unions, `unknown` vs `any`, type guards.
- **Run:** `node modules/01-language-fundamentals/examples/01_types_and_narrowing.ts`
- **Code:** [examples/01_types_and_narrowing.ts](examples/01_types_and_narrowing.ts)
- **Walkthrough:** `greet` accepts any object with a `name`. `area` switches on `shape.kind`; the `never` assignment in `default` fails to compile when a new variant is added. `describe(value: unknown)` must check `typeof` before using the value, and `isStatus` is a _type guard_: after `if (isStatus(input))` the compiler treats `input` as a `Status`.

```ts
function isStatus(value: unknown): value is Status {
  return typeof value === 'string' && (STATUSES as readonly string[]).includes(value);
}
```

- **Expected output:**

  ```text
  Hello, Ada!
  12
  string of length 3
  number 42
  NaN
  null
  array of 2
  object with keys a
  undefined
  object
  valid status: done
  0 NaN 0 NaN
  true true false
  todo 1
  ```

### 3.2 Collections, value vs reference, iterators, closures

- **Goal:** `Map`/`Set`, the `sort` trap, copying, generators, closures and `this`.
- **Run:** `node modules/01-language-fundamentals/examples/02_collections_and_closures.ts`
- **Code:** [examples/02_collections_and_closures.ts](examples/02_collections_and_closures.ts)
- **Walkthrough:** `sort()` without a comparator compares strings and mutates the array, so `[10, 9, 1, 100]` becomes `[1, 10, 100, 9]`; use `toSorted((a, b) => a - b)`. `const b = a` copies the reference, `{ ...a }` copies one level. A generator (`function*`) produces values lazily, so `take(naturals(), 5)` is fine although `naturals` never ends. A method pulled off its object loses `this`; an arrow-function property keeps it.
- **Expected output:**

  ```text
  [ 'go', 'ts', 'rust', 'c#' ]
  Map(4) { 'go' => 2, 'ts' => 2, 'rust' => 1, 'c#' => 1 }
  [ 1, 10, 100, 9 ]
  [ 1, 9, 10, 100 ]
  [ 100, 10, 9, 1 ] [ 10, 9, 1, 100 ]
  20
  2 2 3
  [ 1, 2, 3, 4, 5 ]
  11 12
  11
  Hi, Grace
  TypeError: lost `this`
  ```

### 3.3 Errors and results

- **Goal:** `cause`, custom errors, result unions, `unknown` in `catch`, async errors.
- **Run:** `node modules/01-language-fundamentals/examples/03_errors_and_results.ts`
- **Code:** [examples/03_errors_and_results.ts](examples/03_errors_and_results.ts), shared types in [examples/_shared.ts](examples/_shared.ts)
- **Walkthrough:** `new Error(msg, { cause })` keeps the original failure. `parsePort` returns `Result<number, string>`: the caller must test `result.ok` before touching `value`. In `catch (error)` the variable is `unknown`, so `describeFailure` narrows with `instanceof`. `async` functions turn throws into rejected promises; forgetting `await` moves the failure out of your `try`.

```ts
export type Result<T, E> = { ok: true; value: T } | { ok: false; error: E };
```

- **Expected output:**

  ```text
  config is not valid JSON <- SyntaxError
  port 8080
  invalid port: ""
  invalid port: "abc"
  invalid port: "70000"
  loaded { id: 1 }
  not found: task 2 not found
  validation: title
  non-error thrown: a string
  no error caught here
  handled later: not found: task 99 not found
  done
  ```

### 3.4 The Task Tracker domain

- **Goal:** put it together: types, a generic `Repository`, composition, validation of `unknown`, `Map` grouping.
- **Run:** `node modules/01-language-fundamentals/examples/04_task_domain.ts`
- **Code:** [examples/04_task_domain.ts](examples/04_task_domain.ts)
- **Walkthrough:** `InMemoryTasks implements Repository<Task, NewTask>` receives a `Clock` instead of calling `new Date()` itself (composition, and a fixed clock makes the output deterministic). `#items` is a true private field. `validateNewTask(input: unknown)` returns a typed `NewTask` or a `ValidationError` that carries a `fields` map. `groupBy<T, K extends string>` is a reusable generic.
- **Expected output:**

  ```text
  created {"id":1,"title":"Write docs","status":"todo","due_date":null,"tags":["docs"],"owner_id":null,"created_at":"2030-01-01T09:00:00.000Z"}
  created {"id":2,"title":"Ship it","status":"in_progress","due_date":null,"tags":[],"owner_id":null,"created_at":"2030-01-01T09:00:00.000Z"}
  rejected {"title":"must be 1-200 characters","status":"must be one of todo, in_progress, done"}
  rejected {"body":"must be an object"}
  todo [ 'Write docs' ]
  in_progress [ 'Ship it' ]
  task 42 not found
  ```

## 4. Common Pitfalls

| Mistake                                                                                        | Why it happens                                        | Fix                                                             |
| ---------------------------------------------------------------------------------------------- | ----------------------------------------------------- | --------------------------------------------------------------- |
| `[10, 9, 1].sort()` gives `[1, 10, 9]`                                                         | default order compares strings, and it sorts in place | `toSorted((a, b) => a - b)`                                     |
| `typeof null === 'object'`                                                                     | historical quirk                                      | test `value === null` first                                     |
| `any` leaks: `JSON.parse(text)` is typed `any`, so `const t: Task = JSON.parse(text)` compiles | `any` disables checking silently                      | assign to `unknown` and validate (zod, module 03/04)            |
| `==` or `Number('')` surprises (`Number('')` is `0`, `Number(undefined)` is `NaN`)             | implicit coercion                                     | `===`, explicit conversion, check the text before converting    |
| Mutating a shared object (`const b = a; b.n = 2` changes `a`)                                  | assignment copies the reference                       | spread `{ ...a }`, `structuredClone`, `readonly` types          |
| `const copy = [...items]` only copies one level                                                | spread is shallow                                     | `structuredClone` for nested data                               |
| Passing `obj.method` as a callback loses `this`                                                | `this` is decided by the call site                    | arrow function, `.bind`, or avoid `this`                        |
| Forgetting `await` in a `try` block                                                            | the rejected promise escapes the `catch`              | `await` (or `return`) every promise; lint for floating promises |
| `catch (e)` then `e.message`                                                                   | `e` is `unknown` (anything can be thrown)             | `if (e instanceof Error)`                                       |
| Relative import without extension (`./x`) fails at runtime                                     | Node does no extension lookup in ESM                  | write `./x.ts`                                                  |

## 5. Coming From Other Languages

- **JavaScript:** TypeScript is the same language plus types; the runtime behaviour (coercion, `this`, event loop) is unchanged.
- **Java / C#:** typing is structural, not nominal; interfaces need no `implements`. Generics are erased (no `typeof T` at runtime) and `instanceof` works only on classes, so use discriminated unions for variants.
- **Python:** annotations are checked by `tsc` before running, but nothing validates data at runtime; `unknown` plays the role of "untrusted input".
- **Go:** errors are thrown rather than returned; use `Result` unions when you want Go-style explicit handling. One thread plus an event loop replaces goroutines.

## 6. Exercises

Each exercise has starter code and tests in `exercises/`; reference solutions are in `solutions/` at the same relative paths.

| #   | Title                 | Difficulty | Skill practiced                                     | Run tests                                                                               |
| --- | --------------------- | ---------- | --------------------------------------------------- | --------------------------------------------------------------------------------------- |
| 1   | Collection helpers    | Easy       | `sort` trap, `Set`, `Map`, generics                 | `node --test "modules/01-language-fundamentals/exercises/ex1_collections/**/*.test.ts"` |
| 2   | Results and errors    | Medium     | unions, narrowing `unknown`, `cause`, custom errors | `node --test "modules/01-language-fundamentals/exercises/ex2_results/**/*.test.ts"`     |
| 3   | LRU cache and memoize | Stretch    | generic class, private fields, iterators, closures  | `node --test "modules/01-language-fundamentals/exercises/ex3_lru_cache/**/*.test.ts"`   |

### Exercise 1: Collection helpers

- **Task:** in [collections.ts](exercises/ex1_collections/collections.ts) implement `sortedNumbers`, `uniqueTags` and `countBy`.
- **Acceptance:** [collections.test.ts](exercises/ex1_collections/collections.test.ts) passes: numeric order, the input is never mutated, tags are normalised and deduplicated keeping first position.
- **Hint:** <details><summary>Show</summary>a `Set` remembers insertion order, and `toSorted` returns a copy.</details>

### Exercise 2: Results and errors

- **Task:** in [results.ts](exercises/ex2_results/results.ts) implement `mapResult`, `tryParseJson` (wrap the `SyntaxError` as `cause` of a `ParseError`), `parseDraft` (narrow `unknown`, report all invalid fields) and `unwrap`.
- **Acceptance:** [results.test.ts](exercises/ex2_results/results.test.ts) passes, including falsy parsed values (`0`, `null`) and the identity of the thrown error.
- **Hint:** <details><summary>Show</summary>test `result.ok` first; TypeScript then knows whether `value` or `error` exists. Arrays are objects too.</details>

### Exercise 3: LRU cache and memoize

- **Task:** in [lru-cache.ts](exercises/ex3_lru_cache/lru-cache.ts) implement `LruCache<K, V>` (capacity check, `get` refreshes recency, `peek` does not, eviction, iteration oldest to newest) and `memoize`.
- **Acceptance:** [lru-cache.test.ts](exercises/ex3_lru_cache/lru-cache.test.ts) passes, including `undefined` values and object keys.
- **Hint:** <details><summary>Show</summary>a `Map` iterates in insertion order: delete and re-insert a key to make it the newest, and the first key is the oldest.</details>

## 7. Recap

- Types are erased: they check your code, not your input. Validate `unknown`, never trust `any`.
- Model variants with discriminated unions; let the compiler prove you handled them all (`never`).
- Objects, arrays, `Map` and `Set` are references; copy deliberately and prefer `readonly`, `toSorted`, spread.
- Throw `Error` subclasses (with `cause`) for the unexpected, return result unions for the expected, and always `await`.
- Reach for `Map`/`Set` over plain objects for dynamic keys, and generators when a sequence should be lazy.

## 8. Self-check Questions

1. Why does `const t: Task = JSON.parse(text)` compile, and what would you do instead?
2. What does the `never` assignment in the `default` branch of `area` protect you from?
3. `array.sort()` returns `[1, 10, 9]` for `[10, 9, 1]`. Give two reasons the call is risky.
4. Why is a method passed as a callback different from an arrow-function property?
5. When would you return a `Result` instead of throwing?

## 9. Further Reading

- [TypeScript Handbook](https://www.typescriptlang.org/docs/handbook/intro.html): narrowing, generics, everyday types
- [Node.js: Running TypeScript natively](https://nodejs.org/api/typescript.html)
- [MDN: JavaScript language overview](https://developer.mozilla.org/en-US/docs/Web/JavaScript)

---

**Next:** [02 HTTP Server](../02-http-server/README.md)
