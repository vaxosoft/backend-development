using System.Runtime.CompilerServices;

namespace M14.Exercises;

public class Ex3_DockerfileLinterTests
{
    private const string Good = """
        # syntax=docker/dockerfile:1
        FROM mcr.microsoft.com/dotnet/sdk:10.0 AS build
        WORKDIR /src
        COPY *.csproj ./
        RUN dotnet restore
        COPY . ./
        RUN dotnet publish -c Release -o /out

        FROM mcr.microsoft.com/dotnet/aspnet:10.0
        COPY --from=build /out /app
        USER app
        ENTRYPOINT ["dotnet", "/app/App.dll"]
        """;

    private static string[] Rules(string dockerfile) =>
        Ex3_DockerfileLinter.Lint(dockerfile).Select(f => $"{f.Rule}@{f.Line}").ToArray();

    [Fact]
    public void AGoodMultiStageDockerfile_HasNoFindings() => Assert.Empty(Ex3_DockerfileLinter.Lint(Good));

    [Fact]
    public void TheRepositoryDockerfile_PassesItsOwnLinter()
    {
        var path = Path.Combine(Path.GetDirectoryName(ThisFile())!, "..", "examples", "Dockerfile");

        Assert.True(File.Exists(path), $"expected {path}");
        Assert.Empty(Ex3_DockerfileLinter.Lint(File.ReadAllText(path)));
    }

    [Theory]
    [InlineData("FROM mcr.microsoft.com/dotnet/aspnet\nUSER app\nENTRYPOINT [\"a\"]", "DF001@1")]
    [InlineData("FROM mcr.microsoft.com/dotnet/aspnet:latest\nUSER app\nENTRYPOINT [\"a\"]", "DF001@1")]
    [InlineData("FROM --platform=linux/amd64 alpine\nUSER app\nENTRYPOINT [\"a\"]", "DF001@1")]
    public void UntaggedOrLatestBaseImages_AreFlagged(string dockerfile, string expected) =>
        Assert.Equal([expected], Rules(dockerfile));

    [Fact]
    public void PinnedDigestsScratchStageNamesAndVariablesAreFine()
    {
        const string text = """
            ARG BASE=mcr.microsoft.com/dotnet/aspnet:10.0
            FROM alpine@sha256:0123456789abcdef AS tools
            FROM scratch AS empty
            FROM tools AS final
            FROM ${BASE}
            USER 1000
            ENTRYPOINT ["a"]
            """;

        Assert.Empty(Rules(text));
    }

    [Fact]
    public void ASingleStageSdkImage_IsFlaggedOnTheFinalFrom() =>
        Assert.Equal(["DF002@1"], Rules("FROM mcr.microsoft.com/dotnet/sdk:10.0\nUSER app\nENTRYPOINT [\"dotnet\"]"));

    [Theory]
    [InlineData("")]
    [InlineData("USER root\n")]
    [InlineData("USER 0\n")]
    [InlineData("USER root:root\n")]
    [InlineData("USER app\nUSER root\n")]
    public void RootOrMissingUser_IsFlaggedOnTheFinalFrom(string userLines) =>
        Assert.Equal(["DF003@1"], Rules("FROM alpine:3.20\n" + userLines + "ENTRYPOINT [\"a\"]"));

    [Fact]
    public void OnlyTheFinalStageNeedsANonRootUser()
    {
        const string text = "FROM alpine:3.20 AS build\nRUN true\nFROM alpine:3.20\nUSER app\nENTRYPOINT [\"a\"]";

        Assert.Empty(Rules(text));
    }

    [Theory]
    [InlineData("COPY bin/ /app/")]
    [InlineData("COPY ./obj/Release /app/")]
    [InlineData("COPY --chown=app bin /app")]
    [InlineData("COPY src/a.cs bin/b.dll /app/")]
    public void CopyingHostBuildOutput_IsFlagged(string copy) =>
        Assert.Equal(["DF004@2"], Rules("FROM alpine:3.20\n" + copy + "\nUSER app\nENTRYPOINT [\"a\"]"));

    [Fact]
    public void CopyingFromAnotherStageOrABinLikeNameIsFine()
    {
        var text = "FROM alpine:3.20 AS build\nFROM alpine:3.20\nCOPY --from=build /bin/tool /usr/bin/\nCOPY binary.txt cabin/ /x/\nUSER app\nENTRYPOINT [\"a\"]";

        Assert.Empty(Rules(text));
    }

    [Theory]
    [InlineData("", "DF005@1")]
    [InlineData("ENTRYPOINT dotnet app.dll\n", "DF005@3")]
    [InlineData("CMD dotnet app.dll\n", "DF005@3")]
    public void MissingOrShellFormEntrypoint_IsFlagged(string tail, string expected) =>
        Assert.Equal([expected], Rules("FROM alpine:3.20\nUSER app\n" + tail));

    [Fact]
    public void CmdInExecFormSatisfiesTheEntrypointRule() =>
        Assert.Empty(Rules("FROM alpine:3.20\nUSER app\nCMD [\"dotnet\", \"app.dll\"]"));

    [Fact]
    public void AddOfLocalFiles_IsFlaggedButUrlsAreNot()
    {
        const string text = "FROM alpine:3.20\nADD app.tar.gz /app/\nADD https://example.com/x.txt /x\nUSER app\nENTRYPOINT [\"a\"]";

        Assert.Equal(["DF006@2"], Rules(text));
    }

    [Fact]
    public void ContinuationsCommentsAndCaseAreHandled()
    {
        const string text = """
            # comment
            from alpine:3.20 as final

            copy \
              # a comment inside a continuation
              bin/ \
              /app/
            user app
            entrypoint ["a"]
            """;

        Assert.Equal(["DF004@4"], Rules(text));
    }

    [Fact]
    public void FindingsAreOrderedByLineThenRule()
    {
        const string text = "FROM mcr.microsoft.com/dotnet/sdk\nADD x /x\nENTRYPOINT run";

        Assert.Equal(["DF001@1", "DF002@1", "DF003@1", "DF006@2", "DF005@3"], Rules(text));
    }

    private static string ThisFile([CallerFilePath] string path = "") => path;
}
