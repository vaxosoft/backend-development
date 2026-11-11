#pragma warning disable CS9113 // starter: the constructor parameter is for your implementation to use

using Microsoft.AspNetCore.Diagnostics;
using Microsoft.AspNetCore.Http;
using Microsoft.Extensions.Logging;

namespace M10.Exercises;

public sealed class NotFoundException(string message) : Exception(message);

public sealed class ValidationFailedException(IReadOnlyDictionary<string, string[]> errors) : Exception("validation failed")
{
    public IReadOnlyDictionary<string, string[]> Errors { get; } = errors;
}

/// <summary>
/// Turns exceptions into RFC 9457 <c>application/problem+json</c> responses and logs them at the right level.
/// Always returns true (the exception is handled) and sets <c>HttpContext.Response.StatusCode</c>.
/// <list type="table">
/// <item><term>NotFoundException</term><description>404, title "Not found", detail = exception message; log Warning without the exception.</description></item>
/// <item><term>ValidationFailedException</term><description>422, title "Validation failed", member <c>errors</c> = the field errors; log Warning without the exception.</description></item>
/// <item><term>OperationCanceledException while <c>RequestAborted</c> is cancelled</term><description>status 499, no body, log Debug only (the client left).</description></item>
/// <item><term>anything else</term><description>500, title "An unexpected error occurred", NO detail and nothing from the exception message in the body;
/// log Error with the exception attached and the request method and path as structured values.</description></item>
/// </list>
/// Every problem body has <c>status</c>, <c>title</c>, and <c>request_id</c> when the request carried an <c>X-Request-ID</c> header.
/// </summary>
public sealed class ProblemExceptionHandler(ILogger<ProblemExceptionHandler> logger) : IExceptionHandler
{
    public ValueTask<bool> TryHandleAsync(HttpContext httpContext, Exception exception, CancellationToken cancellationToken)
    {
        throw new NotImplementedException();
    }
}
