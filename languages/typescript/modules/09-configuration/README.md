# 09. Configuration and Environment in TypeScript

> Estimated time: 1-1.5 h | Difficulty: Beginner-Intermediate | Prerequisites: 01 (types, errors), 04 (zod)

## Learning Outcomes

After this module you can:

- [ ] Read environment variables, files and command-line flags and convert them to typed values
- [ ] Build one validated, typed `Config` object at startup and fail fast with a readable list of problems
- [ ] Layer sources with a clear precedence (defaults < file < env < flags) and report where each value came from
- [ ] Keep secrets out of logs and read them from files or the environment
- [ ] Wire an application manually from its configuration (composition root) so tests can swap parts

## 1. Concept Overview

Configuration is everything that differs between environments (dev, CI, production) but not between releases: ports, URLs, log levels, credentials. The [twelve-factor](https://12factor.net/config) approach keeps it in the **environment**, not in code. Real services layer several sources: built-in defaults, an optional file, environment variables and command-line flags, with later sources winning.

Two rules matter more than the mechanism. First, **parse and validate once at startup** into a typed object and pass that object around; code that reads raw `process.env` in twenty places cannot be validated or tested. Second, **fail fast**: a missing `DATABASE_URL` should stop the process at boot with a clear message, not surface as an obscure error on the first request. Secrets are configuration too, but they must never reach a log line.

```
defaults -> config file -> .env / environment -> flags  ==> validate ==> Config (typed, frozen)
                                                                      ==> composition root wires services
```

## 2. How TypeScript Approaches It

| Piece                                                                     | Use                                                                                                    |
| ------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------ |
| `process.env`                                                             | environment: every value is `string \| undefined`                                                      |
| `node:util` `parseArgs`                                                   | declarative flag parsing (`--port 9000`, `-p 9000`, `--flag=value`), strict mode rejects unknown flags |
| `node:util` `parseEnv` / `process.loadEnvFile()` / `node --env-file=.env` | read `.env` files (`parseEnv` returns an object and mutates nothing)                                   |
| `JSON.parse` of a file                                                    | a config file; JSON needs no dependency                                                                |
| `zod`                                                                     | one schema gives runtime validation, coercion and the static type                                      |

Idiomatic style: a pure function `loadConfig(env, argv, fileText): Config` that takes its inputs as parameters (so tests pass plain objects, not a mutated global), returns a typed value, and throws one error listing **all** problems. Types are erased at runtime, so `process.env.PORT as unknown as number` does nothing: validate and convert. No dotenv/convict: layering is a few lines you can read ([PROFILE](../../PROFILE.md)).

## 3. Guided Examples

Run all commands from `languages/typescript/`.

### 3.1 Environment variables are strings

- **Goal:** see why naive conversions fail and write strict parsers.
- **Run:** `node modules/09-configuration/examples/01_env_basics.ts`
- **Code:** [`examples/01_env_basics.ts`](examples/01_env_basics.ts)

```ts
Boolean('false'); // true  <- non-empty string
Number(undefined); // NaN
Number(''); // 0
Number.parseInt('8080abc', 10); // 8080
```

- **Walkthrough:** `parseBool` accepts a closed set of words, `parseIntInRange` uses `Number()` plus `Number.isInteger` and range checks, `parseList` trims and drops empty items. A bad value throws an error naming the variable (`TIMEOUT_MS`). The example passes its own `env` object instead of mutating `process.env`.
- **Expected output:**

  ```
  naive boolean : true <- the string "false" is truthy
  naive number  : NaN <- Number(undefined) is NaN
  naive number  : 0 <- Number("") is 0
  parseInt      : 8080 <- silently accepts trailing junk
  PORT          : 8080
  DEBUG         : false
  LOG_JSON      : true (default)
  TAGS          : [ 'docs', 'p1', 'urgent' ]
  fail fast     : TIMEOUT_MS: expected an integer 1-60000, got "2.5s"
  ```

### 3.2 A validated config object with zod

- **Goal:** one schema produces defaults, coercion, validation and the `Config` type.
- **Run:** `node modules/09-configuration/examples/02_zod_config.ts`
- **Code:** [`examples/02_zod_config.ts`](examples/02_zod_config.ts)

```ts
const envSchema = z.object({
  PORT: z.coerce.number().int().min(1).max(65535).default(8080),
  LOG_LEVEL: z.enum(['debug', 'info', 'warn', 'error']).default('info'),
  DEBUG: z.stringbool().default(false),
  DATABASE_URL: z.url(),
});
```

- **Walkthrough:** `.transform` maps `UPPER_SNAKE` env names to a `camelCase` `Config`; `z.output<typeof schema>` is its type. `z.coerce.boolean()` is a trap (`"false"` becomes `true`); `z.stringbool()` understands `true/false/1/0/yes/no/on/off`. `safeParse` collects every issue, so the operator fixes all of them in one go.
- **Expected output:**

  ```
  {
    port: 8080,
    logLevel: 'info',
    environment: 'development',
    debug: false,
    databaseUrl: 'sqlite:///tracker.db'
  }
  {
    port: 9090,
    logLevel: 'debug',
    environment: 'production',
    debug: false,
    databaseUrl: 'postgres://app@db.internal/tracker'
  }
  coerce.boolean("false") = true
  stringbool("false")     = false
  invalid configuration:
    PORT: Too big: expected number to be <=65535
    LOG_LEVEL: Invalid option: expected one of "debug"|"info"|"warn"|"error"
    DEBUG: Invalid option: expected one of "true"|"1"|"yes"|"on"|"y"|"enabled"|"false"|"0"|"no"|"off"|"n"|"disabled"
    DATABASE_URL: Invalid input: expected string, received undefined
  ```

### 3.3 Layered configuration

- **Goal:** defaults < JSON file < `.env` < environment < flags, with provenance.
- **Run:** `node modules/09-configuration/examples/03_layered_config.ts`
- **Code:** [`examples/03_layered_config.ts`](examples/03_layered_config.ts) with [`config.json`](examples/config.json) and [`sample.env`](examples/sample.env)

```ts
const layerSchema = settingsSchema.partial().strict(); // typos are errors
const { values } = parseArgs({
  args: argv,
  options: { port: { type: 'string', short: 'p' } },
  strict: true,
});
```

- **Walkthrough:** every layer is parsed into the same _partial, strict_ shape, so `--prot 1` or `"prot": 1` in the file is rejected instead of ignored. `merge` applies layers in order and records the winning source for each key. The final object is validated once more. A real program passes `process.env` and `process.argv.slice(2)`; the example passes literals so output is reproducible.
- **Expected output:**

  ```
  defaults only: { port: 8080, logLevel: 'info', dataDir: './data' }
    from  : { port: 'default', logLevel: 'default', dataDir: 'default' }
  + config file: { port: 9000, logLevel: 'warn', dataDir: '/var/lib/tracker' }
    from  : { port: 'file', logLevel: 'file', dataDir: 'file' }
  + .env file: { port: 9000, logLevel: 'debug', dataDir: '/srv/tracker data' }
    from  : { port: 'file', logLevel: '.env', dataDir: '.env' }
  + process env: { port: 9100, logLevel: 'debug', dataDir: '/srv/tracker data' }
    from  : { port: 'env', logLevel: '.env', dataDir: '.env' }
  + flags: { port: 9200, logLevel: 'error', dataDir: '/srv/tracker data' }
    from  : { port: 'flag', logLevel: 'flag', dataDir: '.env' }
  rejected: Unknown option '--prot'
  rejected: Invalid input: expected number, received NaN
  ```

### 3.4 Secrets

- **Goal:** keep secrets out of logs by construction and read them from files (`*_FILE`).
- **Run:** `node modules/09-configuration/examples/04_secrets.ts`
- **Code:** [`examples/04_secrets.ts`](examples/04_secrets.ts)

```ts
class Secret {
  readonly #value: string;
  toJSON() {
    return '[REDACTED]';
  }
  toString() {
    return '[REDACTED]';
  }
  [inspect.custom]() {
    return 'Secret([REDACTED])';
  }
  reveal() {
    return this.#value;
  }
}
```

- **Walkthrough:** the secret lives in a `#private` field, so `console.log`, template strings and `JSON.stringify` all print `[REDACTED]`; the only way out is the explicit `reveal()`. `readSecret` prefers `NAME_FILE` (Docker/Kubernetes mount secrets as files) over `NAME`. Never commit `.env` files with real values, never put secrets in command-line flags (they show up in `ps`).
- **Expected output:**

  ```
  console.log  : {
    port: 8080,
    dbPassword: Secret([REDACTED]),
    apiKey: Secret([REDACTED])
  }
  JSON         : {"port":8080,"dbPassword":"[REDACTED]","apiKey":"[REDACTED]"}
  template     : password=[REDACTED]
  missing      : undefined
  length only  : 16 characters read from the file
  ```

### 3.5 Task Tracker: wiring from config

- **Goal:** manual dependency injection with a composition root, swappable parts and no framework.
- **Run:** `node modules/09-configuration/examples/05_task_tracker_wiring.ts`
- **Code:** [`examples/05_task_tracker_wiring.ts`](examples/05_task_tracker_wiring.ts)

```ts
function buildApp(config: Config, overrides: Overrides = {}): FastifyInstance {
  const store = overrides.store ??
    (config.store === 'file' ? new JsonFileTaskStore(config.dataFile) : new MemoryTaskStore());
  const service = new TaskService({ store, clock: overrides.clock ?? { now: () => new Date() }, logger: ... });
  ...
}
```

- **Walkthrough:** `TaskService` depends only on the small interfaces `TaskStore`, `Clock` and `Logger`; `buildApp` is the single place that picks concrete classes from `config`. Tests pass `overrides` (fixed clock, capturing logger). The `/config` route returns the config with the token redacted. `app.inject()` calls the app without opening a port. A DI container is rarely needed in TypeScript: functions and constructor arguments are enough.
- **Expected output:**

  ```
  memory 201 {
    id: 1,
    title: 'Write docs',
    status: 'todo',
    created_at: '2030-01-01T09:00:00Z'
  }
    list  : 1
    config: [REDACTED]
  file 201 {
    id: 1,
    title: 'Write docs',
    status: 'todo',
    created_at: '2030-01-01T09:00:00Z'
  }
    list  : 1
    config: [REDACTED]
  logged: [ 'created task 1', 'created task 1' ]
  file store persisted: 1
  ```

## 4. Common Pitfalls

| Mistake                                                | Why it happens                                                                                             | Fix                                                                                 |
| ------------------------------------------------------ | ---------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------- |
| `if (process.env.DEBUG)` or `Boolean(env.DEBUG)`       | every env value is a string, so `"false"` and `"0"` are truthy                                             | parse a closed set of words (`z.stringbool()`, own `parseBool`)                     |
| `z.coerce.boolean()`                                   | it applies JS truthiness to the string                                                                     | `z.stringbool()`                                                                    |
| `Number(env.PORT)` / `parseInt`                        | `NaN` for undefined, `0` for `""`, `parseInt('80x')` is `80`                                               | `z.coerce.number().int().min().max()` or a strict regex                             |
| `process.env.X` read all over the code base            | convenient, and it type-checks as `string \| undefined`                                                    | read it once in `loadConfig`; pass the typed `Config` down                          |
| `process.env.PORT = 3000` / `= undefined` in tests     | `process.env` is global shared state and stores everything as a string (`undefined` becomes `"undefined"`) | make `loadConfig(env)` take the env as a parameter                                  |
| Defaults hidden at the use site (`env.PORT \|\| 8080`) | `\|\|` also replaces `"0"`/empty values; defaults are scattered                                            | declare defaults in the schema; use `??` only where `undefined` is the only "unset" |
| Failing on the first problem only                      | `parse()` throws at the first issue                                                                        | `safeParse` and report every issue, each prefixed with the variable name            |
| Typos silently ignored (`LOG_LVL=debug`)               | unknown keys are skipped by default                                                                        | strict schemas for files and flags; warn about unknown prefixed env vars            |
| Logging the config object                              | `console.log(config)` prints every secret                                                                  | wrap secrets in a redacting type or log `redact(config)`                            |
| Committing `.env`                                      | it is convenient for local development                                                                     | commit `.env.example` with fake values and list `.env` in `.gitignore`              |

## 5. Coming From Other Languages

- **JavaScript:** the same `process.env`, but add runtime validation; TypeScript types do not exist at runtime.
- **Java (Spring) / C#:** there is no `application.yml` binding or `IOptions<T>`; `zod` plays the role of the binder plus validation attributes. Constructor injection is the same, only without a container.
- **Python:** like `pydantic-settings`: a schema class reads env and validates. `zod` is the TypeScript equivalent, with a hand-written layering step.
- **Go:** like `envconfig`/`viper` but typed through schemas instead of struct tags; the composition root is `main()` in both.

## 6. Exercises

Starters live in [`exercises/`](exercises/), reference solutions in [`solutions/`](solutions/). Run from `languages/typescript/`. To check a solution, copy its file over the starter (and restore the starter afterwards).

| #   | Title                                                            | Difficulty | Skill practiced                                       | Run tests                                                                       |
| --- | ---------------------------------------------------------------- | ---------- | ----------------------------------------------------- | ------------------------------------------------------------------------------- |
| 1   | [Parse env values](exercises/ex1_parse_env/parse-env.ts)         | Easy       | strict string-to-type conversion, descriptive errors  | `node --test "modules/09-configuration/exercises/ex1_parse_env/*.test.ts"`      |
| 2   | [Zod config](exercises/ex2_zod_config/load-config.ts)            | Medium     | schema, defaults, cross-field rule, all-errors report | `node --test "modules/09-configuration/exercises/ex2_zod_config/*.test.ts"`     |
| 3   | [Layered config](exercises/ex3_layered_config/layered-config.ts) | Stretch    | precedence, `parseArgs`, provenance, redaction        | `node --test "modules/09-configuration/exercises/ex3_layered_config/*.test.ts"` |

**Exercise 1 - Parse env values**

- **Task:** implement `parseBoolean`, `parseInteger`, `parseDuration` and `parseList`; errors are `ConfigError`s that name the variable.
- **Acceptance:** all tests in `parse-env.test.ts` pass.
- <details><summary>Hint</summary>Match integers with a regex before calling `Number()`; keep a small lookup table for the duration units.</details>

**Exercise 2 - Zod config**

- **Task:** implement `loadConfig(env)` for the Task Tracker service (the P7 variables plus `DATABASE_URL`, `ALLOWED_ORIGINS` and `API_TOKEN`), reporting every problem as `VARIABLE: message`.
- **Acceptance:** all tests in `load-config.test.ts` pass, including "production requires a token" and "the token is never echoed".
- <details><summary>Hint</summary>Use `.refine(fn, { path: ['API_TOKEN'], message })` on the object for the cross-field rule, and map `result.error.issues` to strings with `issue.path.join('.')`.</details>

**Exercise 3 - Layered config**

- **Task:** implement `resolveSettings({ file, env, argv })` with precedence defaults < file < env < flags, strict layers, errors naming key and layer, per-key `sources`; plus `redact`.
- **Acceptance:** all tests in `layered-config.test.ts` pass.
- <details><summary>Hint</summary>Convert each layer to the same `Partial<Settings>` first (validating it there), then merge them in a loop while recording the source of each key.</details>

## 7. Recap

- Environment values are strings: convert them strictly, once, into a typed object and fail fast with _all_ problems listed.
- Precedence is defaults < file < env < flags; keep the layers explicit and record where values came from.
- Take `env`/`argv` as function parameters: it makes configuration trivial to test and avoids global state.
- Secrets get a redacting wrapper, come from the environment or `*_FILE`, and never from flags or git.
- Wire the app in one composition root with constructor arguments; reach for a DI framework only when manual wiring really hurts.

## 8. Self-check Questions

1. Why does `z.coerce.boolean().parse("false")` return `true`, and what do you use instead?
2. What is the benefit of `loadConfig(env)` taking the environment as a parameter rather than reading `process.env`?
3. A value is set in the config file, in `.env` and as a flag. Which wins, and how would you find out from logs where the winner came from?
4. Why should unknown config keys be errors, and where does that bite if you let them pass?
5. In example 3.5, what would you change in `buildApp` to test the service with a clock fixed at midnight?

## 9. Further Reading

- [Node.js docs: `util.parseArgs`, `util.parseEnv`, `process.loadEnvFile`](https://nodejs.org/api/util.html#utilparseargsconfig)
- [Zod docs: `z.stringbool`, coercion, `safeParse` and error formatting](https://zod.dev)
- [The Twelve-Factor App: Config](https://12factor.net/config)

---

**Next:** [10 Logging and Diagnostics](../10-logging-and-diagnostics/README.md)
