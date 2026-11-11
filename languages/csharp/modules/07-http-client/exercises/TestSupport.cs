using System.Net;
using System.Net.Http.Headers;

namespace M07.Exercises;

/// <summary>Plays back a script of behaviours, one per call (the last one repeats), and records every request.</summary>
public sealed class ScriptedHandler(params Func<HttpRequestMessage, CancellationToken, Task<HttpResponseMessage>>[] script)
    : HttpMessageHandler
{
    private int _calls;

    public int Calls => Volatile.Read(ref _calls);

    public List<(HttpMethod Method, Uri? Uri, string Body, HttpRequestHeaders Headers)> Seen { get; } = [];

    public static Func<HttpRequestMessage, CancellationToken, Task<HttpResponseMessage>> Status(HttpStatusCode code, string body = "{}") =>
        (_, _) => Task.FromResult(new HttpResponseMessage(code) { Content = new StringContent(body) });

    public static Func<HttpRequestMessage, CancellationToken, Task<HttpResponseMessage>> Throw(Exception ex) =>
        (_, _) => throw ex;

    public static Func<HttpRequestMessage, CancellationToken, Task<HttpResponseMessage>> Hang() =>
        async (_, ct) =>
        {
            await Task.Delay(Timeout.Infinite, ct);
            return new HttpResponseMessage(HttpStatusCode.OK);
        };

    protected override async Task<HttpResponseMessage> SendAsync(HttpRequestMessage request, CancellationToken cancellationToken)
    {
        var index = Interlocked.Increment(ref _calls) - 1;
        var body = request.Content is null ? "" : await request.Content.ReadAsStringAsync(cancellationToken);
        lock (Seen)
        {
            Seen.Add((request.Method, request.RequestUri, body, request.Headers));
        }

        return await script[Math.Min(index, script.Length - 1)](request, cancellationToken);
    }
}

/// <summary>Content whose body can be produced only once, like a stream coming off a socket.</summary>
public sealed class OneShotContent(string text) : HttpContent
{
    private bool _used;

    protected override async Task SerializeToStreamAsync(Stream stream, TransportContext? context)
    {
        if (_used)
        {
            throw new InvalidOperationException("body already consumed; buffer the content before the first send");
        }

        _used = true;
        await stream.WriteAsync(System.Text.Encoding.UTF8.GetBytes(text));
    }

    protected override bool TryComputeLength(out long length)
    {
        length = -1;
        return false;
    }
}

/// <summary>Tracks whether the response (and therefore its content) was disposed.</summary>
public sealed class TrackingContent(string text) : HttpContent
{
    public bool Disposed { get; private set; }

    protected override Task SerializeToStreamAsync(Stream stream, TransportContext? context) =>
        stream.WriteAsync(System.Text.Encoding.UTF8.GetBytes(text)).AsTask();

    protected override bool TryComputeLength(out long length)
    {
        length = text.Length;
        return true;
    }

    protected override void Dispose(bool disposing)
    {
        Disposed = true;
        base.Dispose(disposing);
    }
}
