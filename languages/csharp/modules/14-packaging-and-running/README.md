# 14. Packaging and Running in C#

> Estimated time: 1.5-2h | Difficulty: Intermediate | Prerequisites: 02 (HTTP server), 04 (source-generated JSON), 09 (configuration), 10 (logging)

## Learning Outcomes
After this module you can:
- [ ] Build and publish the service as a framework-dependent app or a self-contained single-file binary (`dotnet publish -r linux-x64`)
- [ ] Stamp a version and commit into the binary and read it at runtime
- [ ] Write a multi-stage `Dockerfile` that is cache-friendly, small and runs as non-root
- [ ] Describe a CI pipeline (format, build with analyzers, test, publish, image)
- [ ] Resolve runtime flags and production settings with a clear precedence (flag > environment > default) and shut down gracefully

## 1. Concept Overview
Packaging turns source into something an operator can run: a binary or a container image, with an identity (version, commit), configured from the outside (flags and environment variables, never rebuilt per environment), that starts fast, answers health probes, and stops cleanly on `SIGTERM` so a rolling deploy does not drop requests.

A good pipeline is boring: restore, check formatting, build with warnings as errors, run tests, publish once, and build the image from the same inputs. Everything that changes between environments (address, log level, connection strings, timeouts) is runtime configuration; everything that identifies the build (version, commit) is stamped at build time.

```
source --restore/build/test--> publish output --COPY--> runtime image --run--> ADDR, flags, env, SIGTERM
              (CI)             (framework-dependent        (aspnet, USER app)
                                or self-contained)
```

## 2. How C# Approaches It
| Need | Tool | Notes |
|------|------|-------|
| Build / run | `dotnet build`, `dotnet run --project <dir> -- <args>` | everything after `--` goes to the app |
| Publish, framework-dependent | `dotnet publish -c Release` | small output, needs the .NET runtime on the host (the `aspnet` image provides it) |
| Publish, self-contained | `-r linux-x64 --self-contained` | bundles the runtime; no install needed on the host |
| Single file | `-p:PublishSingleFile=true` | one executable; combine with `--self-contained`. About 100 MB for this app |
| Cross-compile | `-r linux-x64 / linux-arm64 / win-x64 / osx-arm64` | the SDK on any OS can publish for any runtime identifier (RID) |
| Version stamp | `<Version>` + `-p:SourceRevisionId=<sha>` | becomes `AssemblyInformationalVersion` = `1.4.2+<sha>` |
| Trimming / Native AOT | `PublishTrimmed`, `PublishAot` | advanced; see the pitfalls. Only mentioned here (out of scope) |
| Container | official images `mcr.microsoft.com/dotnet/sdk:10.0` (build) and `aspnet:10.0` (run) | the `aspnet` image has a non-root user `app` (UID 1654) |
| CI | GitHub Actions: `actions/setup-dotnet`, `dotnet format --verify-no-changes`, `dotnet build`, `dotnet test`, `dotnet publish` | "lint" = formatter + Roslyn analyzers with `TreatWarningsAsErrors` (already in `Directory.Build.props`) |
| Runtime config | `ADDR`/`ASPNETCORE_URLS`, `ASPNETCORE_ENVIRONMENT`, `Section__Key` env vars, command-line flags | via `IConfiguration` (module 09) |

Idiomatic: one `dotnet publish` produces the artifact you test and ship; the version comes from MSBuild properties, not a hand-edited file; the service reads `ADDR` and flags, logs to stdout, and honours `SIGTERM` through `IHostApplicationLifetime`/`HostOptions.ShutdownTimeout`. Non-idiomatic: building inside the runtime image, copying a developer's `bin/` into an image, per-environment `appsettings.*.json` baked with secrets.

## 3. Guided Examples
All commands run from `languages/csharp/`. Examples are dispatched by name: `dotnet run --project modules/14-packaging-and-running/examples -- <name>`.

### 3.1 Build info and `dotnet publish`
- **Goal:** read the version stamped at build time; produce a self-contained single file
- **Run:** `dotnet run --project modules/14-packaging-and-running/examples -- 01-build-info`
- **Code:** [01_BuildInfo.cs](examples/01_BuildInfo.cs); version in [M14.Examples.csproj](examples/M14.Examples.csproj) (`<Version>1.4.2</Version>`)
- **Walkthrough:** the SDK writes `AssemblyInformationalVersion` from `<Version>`; passing `-p:SourceRevisionId=<sha>` (what CI does with `${{ github.sha }}`) appends `+<sha>`. The same binary reports its identity in logs and on `/version`. Publishing for Linux from any OS:
  ```
  dotnet publish modules/14-packaging-and-running/examples -c Release -r linux-x64 --self-contained -p:PublishSingleFile=true -p:SourceRevisionId=abc1234 -o out
  ./out/M14.Examples 01-build-info
  ```
- **Expected output:**
  ```
  assembly: M14.Examples
  informational version: 1.4.2
  version part: 1.4.2
  commit part: (none)
  ```
  With `-p:SourceRevisionId=abc1234` (`dotnet run -p:SourceRevisionId=abc1234 --project ... -- 01-build-info`) or from the published binary above:
  ```
  informational version: 1.4.2+abc1234
  version part: 1.4.2
  commit part: abc1234
  ```

### 3.2 Runtime flags and layered settings
- **Goal:** defaults < environment variables < command-line flags
- **Run:** `dotnet run --project modules/14-packaging-and-running/examples -- 02-runtime-flags`
- **Code:** [02_RuntimeFlags.cs](examples/02_RuntimeFlags.cs)
- **Walkthrough:** `ConfigurationBuilder` stacks providers; later ones win. `AddCommandLine` with a switch-mapping dictionary turns `--shutdown-timeout` into the same key the environment variable uses. Containers set the environment (`ENV ADDR=0.0.0.0:8080`), operators override with flags.
  ```csharp
  .AddInMemoryCollection(defaults).AddInMemoryCollection(environment).AddCommandLine(args, switchMappings)
  ```
- **Expected output:**
  ```
  addr=127.0.0.1:8080
  environment=Development
  shutdown timeout=10s
  log level=Information
  ```
  `ADDR=0.0.0.0:9000 LOG_LEVEL=Warning dotnet run ... -- 02-runtime-flags` (environment layer):
  ```
  addr=0.0.0.0:9000
  environment=Development
  shutdown timeout=10s
  log level=Warning
  ```
  `ADDR=0.0.0.0:9000 dotnet run ... -- 02-runtime-flags --addr 127.0.0.1:7000 --env Production --shutdown-timeout 30` (flags beat the environment):
  ```
  addr=127.0.0.1:7000
  environment=Production
  shutdown timeout=30s
  log level=Information
  ```

### 3.3 Source-generated JSON (publish-friendly serialization)
- **Goal:** serialization metadata at compile time, so trimming/AOT cannot remove what reflection would need
- **Run:** `dotnet run --project modules/14-packaging-and-running/examples -- 03-json-source-gen`
- **Code:** [03_JsonSourceGen.cs](examples/03_JsonSourceGen.cs)
- **Walkthrough:** `[JsonSerializable(typeof(TaskDto))]` on a `JsonSerializerContext` makes the compiler generate the serializer. With only that context as resolver, an unregistered type is *known* to be unserializable (`TryGetTypeInfo` is false) instead of failing in production. Note the naming policy belongs to the `JsonSerializerOptions`: the service sets `SnakeCaseLower` on its HTTP options as well.
- **Expected output:**
  ```
  {"id":1,"title":"Write docs","status":"todo","due_date":"2030-01-31"}
  TaskDto registered: True
  NotRegistered registered: False (add [JsonSerializable])
  ```

### 3.4 Task Tracker: a deployable service with graceful shutdown
- **Goal:** the service you would put in an image: `ADDR`, flags, `/healthz`, `/version`, source-generated JSON, `SIGTERM` handling
- **Run:** `ADDR=127.0.0.1:8080 dotnet run --project modules/14-packaging-and-running/examples -- 04-task-service --shutdown-timeout 5` (Ctrl-C or `kill -TERM` to stop; `--log-level Warning` silences request logs)
- **Code:** [04_TaskService.cs](examples/04_TaskService.cs)
- **Walkthrough:** `CreateSlimBuilder` (fewer defaults, startup-friendly) listens on `ADDR`; `HostOptions.ShutdownTimeout` is how long in-flight requests get after `SIGTERM`. `/slow?ms=1500` lets you watch it: request a slow response, send `SIGTERM`, and the response still completes while new connections are refused.
  ```csharp
  builder.Services.Configure<HostOptions>(o => o.ShutdownTimeout = TimeSpan.FromSeconds(settings.ShutdownTimeoutSeconds));
  ```
- **Expected output** (`B=http://127.0.0.1:8080`):
  ```
  $ curl -s $B/healthz
  {"status":"ok"}
  $ curl -s $B/version
  {"name":"task-service","version":"1.4.2"}
  $ curl -s $B/tasks
  [{"id":1,"title":"Write docs","status":"todo","due_date":"2030-01-31"},{"id":2,"title":"Ship it","status":"in_progress","due_date":null}]
  $ curl -s -w ' [%{http_code}]\n' "$B/slow?ms=1500" &   # then: kill -TERM <pid> after 0.4 s
  $ curl -s -m 2 -o /dev/null -w '%{http_code}\n' $B/healthz   # right after SIGTERM: connection refused
  000
  done [200]                                               # the in-flight request finished
  ```
  (the process exits with code 0 after the slow request completes).

### 3.5 Dockerfile and CI pipeline (written, not run: Docker is not available here)
- **Goal:** read a production-shaped multi-stage Dockerfile and CI workflow
- **Run:** `docker build -f modules/14-packaging-and-running/examples/Dockerfile --build-arg GIT_SHA=$(git rev-parse --short HEAD) -t task-service .` then `docker run --rm -p 8080:8080 task-service` (from `languages/csharp/`; `docker` is needed)
- **Code:** [Dockerfile](examples/Dockerfile), [Dockerfile.dockerignore](examples/Dockerfile.dockerignore), [github-actions-ci.yml](examples/github-actions-ci.yml)
- **Walkthrough:**
  ```dockerfile
  FROM mcr.microsoft.com/dotnet/sdk:10.0 AS build
  COPY global.json Directory.Build.props Directory.Packages.props ./     # central settings are part of the build
  COPY .../M14.Examples.csproj ...  &&  RUN dotnet restore ...           # cached until dependencies change
  COPY .../examples/ ...            &&  RUN dotnet publish ... -o /out
  FROM mcr.microsoft.com/dotnet/aspnet:10.0 AS runtime                   # no SDK in the final image
  COPY --from=build /out ./
  ENV ADDR=0.0.0.0:8080
  USER app                                                                 # non-root
  ENTRYPOINT ["dotnet", "M14.Examples.dll", "04-task-service"]            # exec form: receives SIGTERM
  ```
  The build context is `languages/csharp` because the project inherits `Directory.Build.props` and `Directory.Packages.props` from there. `Dockerfile.dockerignore` (picked up by BuildKit next to the Dockerfile) keeps `bin/` and `obj/` out of the context. The CI workflow runs on the runner: setup-dotnet from `global.json`, `dotnet format --verify-no-changes`, `dotnet build -c Release` (analyzers + warnings as errors), `dotnet test`, a single-file `publish` uploaded as an artifact, then the image build. Exercise 3 turns these Dockerfile rules into a linter.
- **Expected output:** none (the files are not executed here); a successful `docker run` then serves the same `/healthz` and `/version` responses as 3.4.

## 4. Common Pitfalls
| Mistake | Why it happens | Fix |
|---------|----------------|-----|
| Server only reachable from the host, not the container | the default `127.0.0.1` binds loopback inside the container | `ENV ADDR=0.0.0.0:8080` (or `ASPNETCORE_URLS=http://+:8080`) and `EXPOSE`/`-p` |
| `PublishTrimmed=true` on a web app | trimming removes what reflection needs; ASP.NET Core is not fully trim-compatible. Here the trimmed binary built (19 MB) but crashed at startup with a `TypeLoadException` in Kestrel | do not trim ASP.NET apps; if you need a small/fast binary use source generators and the Native AOT path (`CreateSlimBuilder`, `PublishAot`), out of scope here |
| Reflection-based JSON after trimming | `JsonSerializer.Serialize(obj)` finds types at runtime | `[JsonSerializable]` + `JsonSerializerContext` (3.3); the trim analyzer flags the reflection overloads (IL2026) |
| `COPY bin/ /app` into the image | works on your machine; stale, wrong-OS build output | multi-stage build: publish inside the SDK stage, `COPY --from=build` |
| Docker build context too small | `docker build modules/...` cannot see `Directory.Packages.props` (the central versions) | build from `languages/csharp` with `-f` |
| Restoring after copying all sources | any code change invalidates the restore layer, so every build re-downloads packages | copy `*.csproj` and the central props first, `dotnet restore`, then copy sources |
| `ENTRYPOINT dotnet App.dll` (shell form) | the shell becomes PID 1 and swallows `SIGTERM`, killing in-flight requests after the grace period | exec form `["dotnet", "App.dll"]` |
| Running as root, or shipping the SDK image | the first `FROM` is also the last one | final stage = `aspnet`/`runtime-deps`, `USER app` |
| `Version` edited by hand in a file | forgotten on release, no commit info | `<Version>` in the project and `-p:SourceRevisionId=$GITHUB_SHA` in CI |
| `ShutdownTimeout` left at its default (30 s) while the orchestrator waits 10 s | the orchestrator sends `SIGKILL` first | keep the host timeout below the orchestrator's grace period |

## 5. Coming From Other Languages
- **Go:** `go build` produces a static binary; `dotnet publish -r linux-x64 --self-contained -p:PublishSingleFile=true` is the closest equivalent (bigger, includes the runtime). Cross-compilation is `-r` instead of `GOOS/GOARCH`.
- **Java:** a framework-dependent publish ~ a JAR run by a JRE image; self-contained ~ a `jlink`ed image. `dotnet publish` replaces Maven/Gradle packaging; `mcr.microsoft.com/dotnet/aspnet` ~ a JRE base image.
- **TypeScript/JS:** `dotnet publish` output ~ `npm run build` + production `node_modules`, but the runtime image only needs the compiled output; `ASPNETCORE_ENVIRONMENT` ~ `NODE_ENV`.
- **Python:** no interpreter-less artifact in the box; single-file self-contained is what `pyinstaller` tries to achieve.

## 6. Exercises
Starter code and tests live in [exercises/](exercises/); solutions in [solutions/](solutions/). Docker is not required: everything is plain, fast, offline text and argument processing.

| # | Title | Difficulty | Skill practiced | Run tests |
|---|-------|-----------|-----------------|-----------|
| 1 | Build info | Easy | parse/format `MAJOR.MINOR.PATCH[-pre][+commit]` | `dotnet test modules/14-packaging-and-running/exercises --filter "FullyQualifiedName~.Ex1_"` |
| 2 | Runtime flag parsing | Medium | flag > environment > default, validation with clear errors | `dotnet test modules/14-packaging-and-running/exercises --filter "FullyQualifiedName~.Ex2_"` |
| 3 | Dockerfile linter | Stretch | encode Dockerfile best practices as checkable rules over text | `dotnet test modules/14-packaging-and-running/exercises --filter "FullyQualifiedName~.Ex3_"` |

**Exercise 1 - [Ex1_BuildInfo.cs](exercises/Ex1_BuildInfo.cs)**
- **Task:** `Ex1_BuildInfo.Parse` splits an informational version into version, pre-release and short commit (max 7 characters); `ToDisplayString` formats it; malformed input throws `FormatException`.
- **Acceptance:** the valid table (with/without pre-release, commit, empty `+`), eight malformed inputs, and display strings.
- <details><summary>Hint</summary>Cut at <code>'+'</code> first, then at the first <code>'-'</code>; what remains must be three all-digit parts (<code>char.IsAsciiDigit</code>).</details>

**Exercise 2 - [Ex2_ServerOptions.cs](exercises/Ex2_ServerOptions.cs)**
- **Task:** `Ex2_ServerOptionsParser.Parse(args, environment)` for `--addr`, `--env`, `--shutdown-timeout` (both `--x v` and `--x=v`), with env fallbacks `ADDR`, `ENVIRONMENT`, `SHUTDOWN_TIMEOUT_SECONDS` and defaults; throw `FlagException` naming the flag or variable on any problem.
- **Acceptance:** defaults, environment layer, flag precedence, partial overrides, IPv6 `[::1]:9000`, nine invalid values, unknown flags, missing values; an invalid environment variable is ignored when a valid flag overrides it.
- <details><summary>Hint</summary>Read flags into a dictionary first, then resolve each setting lazily: flag, else environment, else default, and only then validate that winner. <code>LastIndexOf(':')</code> splits <code>host:port</code> even for IPv6.</details>

**Exercise 3 - [Ex3_DockerfileLinter.cs](exercises/Ex3_DockerfileLinter.cs)**
- **Task:** `Lint(text)` returns findings `DF001`-`DF006` (untagged/`latest` base, SDK image in the final stage, root or missing `USER`, copying host `bin/`/`obj/`, shell-form or missing `ENTRYPOINT`/`CMD`, `ADD` of local files) with the 1-based line of the instruction. Handle comments, case, and `\` continuations.
- **Acceptance:** the rule tables, stage names/digests/variables as exceptions, continuation lines, ordering, and the module's own [Dockerfile](examples/Dockerfile) must produce zero findings.
- <details><summary>Hint</summary>First turn the text into (line, KEYWORD, args) instructions, joining continuations; then each rule is a small query. The final stage is everything after the last <code>FROM</code>.</details>

## 7. Recap
- `dotnet publish` is the one build step: framework-dependent for containers, self-contained single-file for standalone binaries; `-r` cross-compiles.
- Stamp identity at build time (`Version` + `SourceRevisionId`); configure behaviour at runtime (`ADDR`, flags, environment).
- Multi-stage Dockerfile: restore layer first, publish in the SDK stage, run in `aspnet` as `USER app`, exec-form `ENTRYPOINT`.
- CI = format + analyzers/warnings-as-errors + tests + publish + image, all from the same commands you run locally.
- Stop gracefully: honour `SIGTERM`, set a `ShutdownTimeout` smaller than the orchestrator's grace period.

## 8. Self-check Questions
1. Why can a framework-dependent publish run on the `aspnet` image, and what changes if you publish self-contained?
2. Where does the `+abc1234` in `1.4.2+abc1234` come from, and why is it better than editing a version file?
3. Why must the `ENTRYPOINT` be in exec form for graceful shutdown, and what does `HostOptions.ShutdownTimeout` add?
4. A teammate runs `docker build modules/14-packaging-and-running/examples` and restore fails with unknown package versions. Why?
5. Which settings belong in the image (`ENV`), which in flags, and which in neither (secrets)?

## 9. Further Reading
- [.NET application publishing overview](https://learn.microsoft.com/dotnet/core/deploying/) and [single-file deployment](https://learn.microsoft.com/dotnet/core/deploying/single-file/overview)
- [Containerize a .NET app / official images](https://learn.microsoft.com/dotnet/core/docker/build-container) and [Dockerfile best practices](https://docs.docker.com/build/building/best-practices/)
- [ASP.NET Core host and graceful shutdown](https://learn.microsoft.com/aspnet/core/fundamentals/host/generic-host) and [GitHub Actions for .NET](https://learn.microsoft.com/dotnet/devops/github-actions-overview)

---
**Next:** [Back to the C# track overview](../../README.md)
