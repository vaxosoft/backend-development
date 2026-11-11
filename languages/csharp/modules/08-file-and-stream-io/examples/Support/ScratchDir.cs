namespace M08.Examples;

/// <summary>A temp directory that deletes itself, so examples leave nothing behind.</summary>
public sealed class ScratchDir : IDisposable
{
    private readonly DirectoryInfo _dir = Directory.CreateTempSubdirectory("m08-");

    public string Path => _dir.FullName;

    public string File(string name) => System.IO.Path.Combine(Path, name);

    public void Dispose() => _dir.Delete(recursive: true);
}
