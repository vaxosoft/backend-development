namespace P5.UrlShortener;

/// <summary>Where finished request log lines go (one JSON object per call). Tests swap it for a list.</summary>
public interface ILogLineSink
{
    void Write(string line);
}

public sealed class ConsoleLogLineSink : ILogLineSink
{
    public void Write(string line) => Console.Out.WriteLine(line);
}
