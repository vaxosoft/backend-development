using System.Text.RegularExpressions;

namespace P7.TestedService.Tests;

public sealed record Instruction(string Name, string Args);

/// <summary>A Dockerfile split into stages (one per FROM). Comments are dropped, line continuations joined.</summary>
public sealed record Dockerfile(IReadOnlyList<IReadOnlyList<Instruction>> Stages)
{
    public static Dockerfile Parse(string text)
    {
        var stages = new List<List<Instruction>>();
        var logical = Regex.Replace(text.Replace("\r\n", "\n"), @"\\\s*\n", " ");
        foreach (var raw in logical.Split('\n'))
        {
            var line = raw.Trim();
            if (line.Length == 0 || line.StartsWith('#'))
            {
                continue;
            }

            var parts = line.Split((char[]?)null, 2, StringSplitOptions.RemoveEmptyEntries);
            var instruction = new Instruction(parts[0].ToUpperInvariant(), parts.Length > 1 ? parts[1] : string.Empty);
            if (instruction.Name == "FROM")
            {
                stages.Add([]);
            }

            if (stages.Count > 0)
            {
                stages[^1].Add(instruction);
            }
        }

        return new Dockerfile(stages);
    }

    public IReadOnlyList<Instruction> Final => Stages[^1];

    /// <summary>The image of a stage's FROM line, ignoring flags such as --platform and the AS alias.</summary>
    public static string ImageOf(IReadOnlyList<Instruction> stage) =>
        stage[0].Args.Split(' ', StringSplitOptions.RemoveEmptyEntries).First(t => !t.StartsWith("--"));
}

/// <summary>Locates and reads the files a learner adds to the starter project.</summary>
public static class RepoFiles
{
    public static string StarterDir()
    {
        for (var dir = new DirectoryInfo(AppContext.BaseDirectory); dir is not null; dir = dir.Parent)
        {
            var candidate = Path.Combine(dir.FullName, "starter");
            if (File.Exists(Path.Combine(candidate, "P7.TestedService.csproj")))
            {
                return candidate;
            }
        }

        throw new InvalidOperationException("starter/ directory not found above " + AppContext.BaseDirectory);
    }

    /// <summary>Fails the test with a helpful message instead of a FileNotFoundException when the file is missing.</summary>
    public static string Read(string relativePath)
    {
        var path = Path.Combine(StarterDir(), relativePath);
        Assert.True(File.Exists(path), $"{relativePath} is missing from the starter project");
        return File.ReadAllText(path);
    }
}
