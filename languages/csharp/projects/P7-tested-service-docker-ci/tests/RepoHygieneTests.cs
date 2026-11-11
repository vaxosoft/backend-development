using System.Text.RegularExpressions;

namespace P7.TestedService.Tests;

/// <summary>Docker is never run here: these tests read the packaging files and assert the rules as text.</summary>
public class RepoHygieneTests
{
    private static Dockerfile LoadDockerfile() => Dockerfile.Parse(RepoFiles.Read("Dockerfile"));

    [Fact]
    public void Dockerfile_builds_in_an_sdk_stage_and_ships_a_runtime_only_stage()
    {
        var dockerfile = LoadDockerfile();

        Assert.True(dockerfile.Stages.Count >= 2, "use a multi-stage build (at least two FROM lines)");
        var build = dockerfile.Stages[0];
        Assert.Contains("dotnet/sdk", Dockerfile.ImageOf(build));
        Assert.Contains(build, i => i.Name == "RUN" && i.Args.Contains("dotnet publish"));
        Assert.Matches(@"dotnet/(aspnet|runtime|runtime-deps)\b", Dockerfile.ImageOf(dockerfile.Final));
        Assert.Contains(dockerfile.Final, i => i.Name == "COPY" && i.Args.StartsWith("--from="));
    }

    [Fact]
    public void Dockerfile_final_stage_runs_as_a_non_root_user()
    {
        var user = LoadDockerfile().Final.LastOrDefault(i => i.Name == "USER");

        Assert.NotNull(user);
        Assert.DoesNotMatch(@"^(root|0)(:|$)", user.Args.Trim());
    }

    [Fact]
    public void Dockerfile_exposes_the_port_and_documents_how_to_probe_health()
    {
        var text = RepoFiles.Read("Dockerfile");
        var final = LoadDockerfile().Final;

        Assert.Contains(final, i => i.Name == "EXPOSE");
        var hasHealthcheck = final.Any(i => i.Name == "HEALTHCHECK");
        var documentsProbe = Regex.IsMatch(text, @"(?im)^\s*#.*probe.*/healthz");
        Assert.True(hasHealthcheck || documentsProbe, "add a HEALTHCHECK, or a comment starting with 'Probe' that names /healthz");
    }

    [Fact]
    public void Dockerfile_never_copies_build_output_from_the_host()
    {
        var copies = LoadDockerfile().Stages.SelectMany(s => s).Where(i => i.Name is "COPY" or "ADD" && !i.Args.StartsWith("--from="));

        Assert.DoesNotContain(copies, i => Regex.IsMatch(i.Args, @"(^|[\s/])(bin|obj)(/|\s|$)"));
    }

    [Fact]
    public void Dockerfile_restores_packages_before_copying_the_sources()
    {
        var build = LoadDockerfile().Stages[0].Select(i => $"{i.Name} {i.Args}").ToList();

        var restore = build.FindIndex(l => l.StartsWith("RUN") && l.Contains("dotnet restore"));
        var copyAll = build.FindIndex(l => Regex.IsMatch(l, @"^COPY\s+\.\s+\S+"));
        Assert.True(restore >= 0, "run dotnet restore in its own layer");
        Assert.True(copyAll > restore, "copy the project file, restore, and only then copy the rest (layer caching)");
    }

    [Fact]
    public void Dockerignore_keeps_build_output_and_git_history_out_of_the_build_context()
    {
        var lines = RepoFiles.Read(".dockerignore").Split('\n').Select(l => l.Trim().TrimEnd('/')).ToList();

        Assert.Contains(lines, l => l is "bin" or "**/bin");
        Assert.Contains(lines, l => l is "obj" or "**/obj");
        Assert.Contains(".git", lines);
    }

    [Fact]
    public void Ci_runs_on_push_and_on_pull_request()
    {
        var ci = RepoFiles.Read(".github/workflows/ci.yml");

        Assert.Matches(@"(?m)^on:", ci);
        Assert.Matches(@"(?m)^\s+push:", ci);
        Assert.Matches(@"(?m)^\s+pull_request:", ci);
    }

    [Fact]
    public void Ci_restores_builds_with_warnings_as_errors_checks_format_and_runs_the_tests()
    {
        var ci = RepoFiles.Read(".github/workflows/ci.yml");

        Assert.Contains("dotnet restore", ci);
        Assert.Contains("dotnet build", ci);
        Assert.Contains("-warnaserror", ci);
        Assert.Contains("dotnet format", ci);
        Assert.Contains("--verify-no-changes", ci);
        Assert.Contains("dotnet test", ci);
    }

    [Fact]
    public void Ci_pins_the_toolchain_and_limits_the_token_to_read_access()
    {
        var ci = RepoFiles.Read(".github/workflows/ci.yml");

        Assert.Contains("actions/setup-dotnet@", ci);
        Assert.Matches(@"dotnet-version:\s*['""]?10\.0", ci);
        Assert.Matches(@"(?ms)^permissions:\s*\n\s+contents:\s*read", ci);
    }
}
