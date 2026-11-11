# 09. Configuration in C#

> Estimated time: 1.5h | Difficulty: Intermediate | Prerequisites: 01, 02

## Learning Outcomes
After this module you can:
- [ ] Layer configuration from JSON files, environment variables and command-line arguments, and predict which value wins.
- [ ] Bind sections to typed options and validate them at startup with a clear error.
- [ ] Choose between `IOptions<T>`, `IOptionsSnapshot<T>` and `IOptionsMonitor<T>`.
- [ ] Keep secrets out of source control and out of logs.
- [ ] Wire a small application by hand and with the DI container.

## 1. Concept Overview
Configuration is everything that differs between environments but not between builds: ports, connection strings, log levels, feature flags, secrets. The twelve-factor rule of thumb is **code reads configuration from layers**: sensible defaults in the repo, per-environment overrides in files, and the deployment's environment variables and command-line arguments on top. The program validates the merged result **once, at startup**, and fails fast with every problem listed, instead of crashing on the first request that touches a bad value.

```
defaults < appsettings.json < appsettings.<Environment>.json < environment variables < command line
                                  (the last provider that has a key wins)
```

## 2. How C# Approaches It
| Piece | Role |
|-------|------|
| `ConfigurationBuilder` / `IConfiguration` | A flat key space with `:` separators (`Server:Port`); each source is a *provider*, later providers override earlier ones |
| Providers | In-memory, JSON, environment (`Section__Key`, optional prefix), command line (`--Section:Key=value`), user-secrets (development only), Azure/Key Vault etc. |
| `WebApplication.CreateBuilder` / `Host.CreateApplicationBuilder` | Already wire the standard order: `appsettings.json`, `appsettings.{Environment}.json`, user-secrets (Development), environment variables, command line |
| Options pattern | `services.AddOptions<T>().BindConfiguration("Section")`; inject `IOptions<T>` (singleton snapshot), `IOptionsSnapshot<T>` (per scope, scoped services only) or `IOptionsMonitor<T>` (live, with `OnChange`) |
| Validation | `ValidateDataAnnotations()`, `Validate(...)`, `IValidateOptions<T>`, and `ValidateOnStart()` |
| DI | `ServiceCollection` + lifetimes; the container is optional: a plain composition root works too |

Idiomatic: typed options objects, bound once; `ValidateOnStart`; secrets from user-secrets/environment/secret store. Non-idiomatic: `Environment.GetEnvironmentVariable` sprinkled through the code, injecting `IConfiguration` into business classes, magic string keys everywhere. No third-party libraries are needed.

## 3. Guided Examples
Run from `languages/csharp/`: `dotnet run --project modules/09-configuration/examples -- <name>`. Examples set their environment variables in-process so the output is reproducible.

### 3.1 Layers and who wins
- **Goal:** see the provider order in action.
- **Run:** `dotnet run --project modules/09-configuration/examples -- 01-layers`
- **Code:** [examples/01_layers.cs](examples/01_layers.cs), files in [examples/config/](examples/config/appsettings.json)
  ```csharp
  new ConfigurationBuilder().SetBasePath(...)
      .AddInMemoryCollection(defaults).AddJsonFile("appsettings.json", optional: false)
      .AddJsonFile($"appsettings.{env}.json", optional: true)
      .AddEnvironmentVariables(prefix: "TT_").AddCommandLine(args).Build();
  ```
- **Walkthrough:** `TT_Server__Port=9090` becomes `Server:Port` (double underscore = `:`; the prefix is stripped). `GetDebugView()` shows which provider supplied each value: great for "why is my port 5000?". Missing keys are `null`, not errors.
- **Expected output:**
  ```
  Server:Host         = 0.0.0.0   (command line beat appsettings.json)
  Server:Port         = 9090   (environment beat Development file, JSON and defaults)
  Logging:Level       = Debug   (Development file beat appsettings.json)
  Server:RequestTimeout = 00:00:30   (only appsettings.json knows it)
  Feature:Beta        = false   (only the in-memory defaults know it)
  Not:There           = <null>   (missing keys are null, not errors)

  Who supplied what:
  Database:
    ConnectionString=Data Source=/var/lib/tt/tasks.db (EnvironmentVariablesConfigurationProvider Prefix: 'TT_')
  Feature:
    Beta=false (MemoryConfigurationProvider)
  Logging:
    Level=Debug (JsonConfigurationProvider for 'appsettings.Development.json' (Optional))
  Server:
    Host=0.0.0.0 (CommandLineConfigurationProvider)
    Port=9090 (EnvironmentVariablesConfigurationProvider Prefix: 'TT_')
    RequestTimeout=00:00:30 (JsonConfigurationProvider for 'appsettings.json' (Required))
  ```

### 3.2 Binding to typed options, and what fails quietly
- **Goal:** bind sections (strings, numbers, `TimeSpan`, enums, arrays, dictionaries) and know the silent failure mode.
- **Run:** `dotnet run --project modules/09-configuration/examples -- 02-binding`
- **Code:** [examples/02_binding.cs](examples/02_binding.cs)
- **Walkthrough:** `config.GetSection("Server").Get<ServerOptions>()` fills properties by name (case-insensitive). A misspelled key (`Prot`) is **ignored** and the default stays, unless you opt in to `ErrorOnUnknownConfiguration`. A value that cannot be converted (`eighty` for an `int`) throws.
- **Expected output:**
  ```
  host=0.0.0.0 port=9090 timeout=00:00:05 level=Warning
  origins=[https://app.example.com, https://admin.example.com] limits=[login:5, search:30]
  GetValue<int>: 9090, with default: 42

  -- a typo is silently ignored
  Port after typo 'Prot': 8080 (the default; no error)
  ErrorOnUnknownConfiguration: 'ErrorOnUnknownConfiguration' was set on the provided BinderOptions, but the following properties were not found on the instance of M09.Examples.ServerOptions: 'Prot'

  -- a value of the wrong type is loud
  InvalidOperationException: Failed to convert configuration value 'eighty' at 'Server:Port' to type 'System.Int32'.
  ```

### 3.3 Validation that stops startup
- **Goal:** collect every problem and fail before serving traffic.
- **Run:** `dotnet run --project modules/09-configuration/examples -- 03-validation`
- **Code:** [examples/03_validation.cs](examples/03_validation.cs)
  ```csharp
  builder.Services.AddOptions<AppOptions>().BindConfiguration("App").ValidateDataAnnotations().ValidateOnStart();
  builder.Services.AddSingleton<IValidateOptions<AppOptions>, AppOptionsRules>(); // cross-property rules
  ```
- **Walkthrough:** attributes (`[Range]`, `[Required]`, `[RegularExpression]`) cover single properties; `IValidateOptions<T>` covers rules such as "Production requires a connection string". Without `ValidateOnStart()` validation runs lazily on first access, so the bad config would surface in the middle of a request.
- **Expected output:**
  ```
  -- broken config
  startup failed (3 problems):
    - DataAnnotation validation failed for 'AppOptions' members: 'Port' with the error: 'The field Port must be between 1 and 65535.'.
    - DataAnnotation validation failed for 'AppOptions' members: 'LogLevel' with the error: 'The field LogLevel must match the regular expression '^(debug|info|warn|error)$'.'.
    - ConnectionString is required when Environment is Production
  -- good config
  started: port=9090 level=warn env=Production
  ```

### 3.4 `IOptions` vs `IOptionsSnapshot` vs `IOptionsMonitor`
- **Goal:** know what each sees after configuration is reloaded.
- **Run:** `dotnet run --project modules/09-configuration/examples -- 04-reload-and-lifetimes`
- **Code:** [examples/04_reload_and_lifetimes.cs](examples/04_reload_and_lifetimes.cs)
- **Walkthrough:** `root.Reload()` stands in for an edited file (`AddJsonFile(..., reloadOnChange: true)` does this for you). `IOptions<T>` is computed once (singleton) and never changes; `IOptionsSnapshot<T>` is recomputed per scope (per request); `IOptionsMonitor<T>` always has the latest value and can notify you.
- **Expected output:**
  ```
  before change: IOptions=20 Snapshot=20 Monitor=20
  -- configuration reloaded
    [monitor.OnChange] PageSize is now 50
  after change:  IOptions=20 Snapshot=50 Monitor=50
  ```

### 3.5 Task Tracker config: validated, redacted, wired
- **Goal:** the shape used by the P7 capstone: environment variables (`PORT`, `LOG_LEVEL`, `ENV`, `JWT_SECRET`) -> a validated immutable `AppConfig`, safe to log, then composition by hand vs DI.
- **Run:** `dotnet run --project modules/09-configuration/examples -- 05-task-tracker-config`
- **Code:** [examples/05_task_tracker_config.cs](examples/05_task_tracker_config.cs)
- **Walkthrough:** `TryLoad` returns **all** errors, not the first; `ToString()` masks the secret; the same `TaskService` is built with `new` in a composition root and with `ServiceCollection` (`ValidateOnBuild`/`ValidateScopes` catch lifetime mistakes at startup).
- **Expected output:**
  ```
  -- invalid environment
  config error: PORT must be an integer between 1 and 65535, got 'http'
  config error: LOG_LEVEL must be one of debug|info|warn|error, got 'verbose'
  config error: JWT_SECRET must be at least 16 characters in production
  -- valid environment
  loaded: Port=9090 LogLevel=warn Environment=production JwtSecret=***
  -- wiring by hand (composition root)
  TaskService with 3 tasks, listening on :9090
  -- wiring with the DI container
  TaskService with 3 tasks, listening on :9090
  ```

**Secrets in practice:** in development `dotnet user-secrets init` and `dotnet user-secrets set "Jwt:Secret" "..."` store values outside the repo (loaded automatically by `CreateBuilder` in the Development environment). In production use environment variables or a secret manager. Never commit them to `appsettings.json` and never print a whole configuration without redaction.

## 4. Common Pitfalls
| Mistake | Why it happens | Fix |
|---------|----------------|-----|
| Env var `Server:Port=...` has no effect | `:` is not valid in most shells/environments | Use `Server__Port` (double underscore); with a prefix, `TT_Server__Port` |
| A typo in a key leaves the default in place | Binding ignores unknown keys silently | `ErrorOnUnknownConfiguration`, validation of required values, and `GetDebugView()` when debugging |
| Validation only fails on first use | `ValidateDataAnnotations()` is lazy | Add `ValidateOnStart()` (or validate eagerly in `Program`) |
| Injecting `IOptionsSnapshot<T>` into a singleton | Snapshot is scoped; the container throws (or, if captured, goes stale) | `IOptionsMonitor<T>` in singletons, `IOptionsSnapshot<T>` in scoped/request code |
| Expecting `IOptions<T>` to see file edits | It is read once | `IOptionsMonitor<T>` / `IOptionsSnapshot<T>` with `reloadOnChange: true` |
| Secrets in `appsettings.json` or in `GetDebugView()` output | Convenient during development | user-secrets, env vars, secret store; redact before logging |
| Reading `IConfiguration["A:B"]` in business code | Stringly typed and untestable | Bind once to a typed options class; inject `IOptions<T>` or the plain object |
| Environment name mismatch (`development` vs `Development`) | File names and comparisons are case-sensitive on Linux | Use the framework's `IHostEnvironment.IsDevelopment()` and exact casing |
| Wrong provider order: environment added before the JSON file | Later providers win, so the file overrides the environment | Keep the standard order; the debug view shows the winner |

## 5. Coming From Other Languages
- **Java/Spring:** `application.yml` + profiles + `@ConfigurationProperties` ~ `appsettings.{Environment}.json` + options pattern; relaxed binding ~ case-insensitive keys and `__`.
- **Go:** `os.Getenv` + `flag` + struct decoding (`envconfig`, Viper) ~ providers + `Bind`; validation is explicit there, built into `ValidateOnStart` here.
- **TypeScript/JS:** `dotenv`/`process.env` + `zod` validation ~ environment provider + `IValidateOptions`; there is no mutable global, config is injected.
- **Python:** `pydantic-settings` (`BaseSettings`) ~ options class + validation; `os.environ` ~ the environment provider.

## 6. Exercises
Run one exercise at a time from `languages/csharp/`. Solutions are in [solutions/](solutions/).

| # | Title | Difficulty | Skill practiced | Run tests |
|---|-------|-----------|-----------------|-----------|
| 1 | Layered config and redacted dump | Easy | Provider order, env-var naming, safe dumps | `dotnet test modules/09-configuration/exercises --filter "FullyQualifiedName~.Ex1_"` |
| 2 | Options validation | Medium | `IValidateOptions<T>`, `ValidateOnStart`, collecting all errors | `dotnet test modules/09-configuration/exercises --filter "FullyQualifiedName~.Ex2_"` |
| 3 | Live settings | Stretch | `IOptionsMonitor<T>`, rejecting invalid reloads, keeping the last good value | `dotnet test modules/09-configuration/exercises --filter "FullyQualifiedName~.Ex3_"` |

### Exercise 1: [Ex1_LayeredConfig.cs](exercises/Ex1_LayeredConfig.cs)
- **Task:** `LayeredConfig.Build` (JSON text < environment JSON text < prefixed env vars < command line) and `DumpRedacted` (sorted `Key=Value` lines, secrets as `***`).
- **Acceptance:** `Ex1_LayeredConfigTests` (precedence, `__` nesting, prefix filtering, redaction rules).
- **Hint:** <details><summary>Show</summary>`AddJsonStream(new MemoryStream(...))` for text sources; `AsEnumerable()` lists all keys, including section nodes that have no value.</details>

### Exercise 2: [Ex2_OptionsValidation.cs](exercises/Ex2_OptionsValidation.cs)
- **Task:** write `AppOptionsValidator` (all rules, every failure reported) and `AddAppOptions` (bind `App`, register the validator, validate on start).
- **Acceptance:** `Ex2_OptionsValidationTests` (host start fails with every problem, boundaries, Production rule, defaults pass).
- **Hint:** <details><summary>Show</summary>Collect messages in a `List<string>` and return `ValidateOptionsResult.Fail(list)`; register the validator with `AddSingleton<IValidateOptions<AppOptions>, ...>()`.</details>

### Exercise 3: [Ex3_LiveSettings.cs](exercises/Ex3_LiveSettings.cs)
- **Task:** `LiveSettings` serves `RateLimitOptions`, versions accepted changes, ignores invalid or unchanged reloads and exposes the last rejection.
- **Acceptance:** `Ex3_LiveSettingsTests` (fail-fast start, valid edit, rejected edit keeps last good, recovery, no-op reload, dispose unsubscribes).
- **Hint:** <details><summary>Show</summary>`monitor.OnChange(callback)` returns an `IDisposable`; copy the values into a new object before validating, and guard updates with a lock.</details>

## 7. Recap
- Configuration is layered; the last provider that defines a key wins, and the debug view tells you who that was.
- Bind to typed options once; validate with `ValidateOnStart` so bad configuration stops the process at boot.
- `IOptions` = fixed, `IOptionsSnapshot` = per scope, `IOptionsMonitor` = live.
- Secrets live in user-secrets, the environment or a secret store, never in the repo or the logs.
- When to reach for what: files for defaults, environment variables for deployment values, command line for one-off overrides.

## 8. Self-check Questions
1. In what order are the default `WebApplication` providers applied, and how would you make a value from `appsettings.json` override an environment variable (and why shouldn't you)?
2. How do you spell the key `Database:ConnectionString` as an environment variable, and why?
3. Binding `Server:Prot=9090` succeeds. What does the options object contain, and how can you make it fail?
4. A singleton service must react to changed configuration. Which options interface do you inject, and what happens if you inject `IOptionsSnapshot<T>`?
5. Why validate at startup rather than in the code that uses the value?

## 9. Further Reading
- [Configuration in .NET](https://learn.microsoft.com/dotnet/core/extensions/configuration) and [Options pattern](https://learn.microsoft.com/dotnet/core/extensions/options)
- [Safe storage of app secrets in development](https://learn.microsoft.com/aspnet/core/security/app-secrets)
- [The Twelve-Factor App: Config](https://12factor.net/config)

---
**Next:** [10 Logging and Diagnostics](../10-logging-and-diagnostics/README.md)
