using Microsoft.AspNetCore.Diagnostics;
using Microsoft.AspNetCore.Http;
using Microsoft.AspNetCore.Mvc;
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
    public async ValueTask<bool> TryHandleAsync(HttpContext httpContext, Exception exception, CancellationToken cancellationToken)
    {
        if (exception is OperationCanceledException && httpContext.RequestAborted.IsCancellationRequested)
        {
            logger.LogDebug("Client disconnected before {Method} {Path} finished", httpContext.Request.Method, httpContext.Request.Path.Value);
            httpContext.Response.StatusCode = 499;
            return true;
        }

        ProblemDetails problem;
        switch (exception)
        {
            case NotFoundException:
                logger.LogWarning("Not found: {Reason}", exception.Message);
                problem = new() { Status = StatusCodes.Status404NotFound, Title = "Not found", Detail = exception.Message };
                break;
            case ValidationFailedException validation:
                logger.LogWarning("Validation failed for {Method} {Path}", httpContext.Request.Method, httpContext.Request.Path.Value);
                problem = new() { Status = StatusCodes.Status422UnprocessableEntity, Title = "Validation failed" };
                problem.Extensions["errors"] = validation.Errors;
                break;
            default:
                logger.LogError(exception, "Unhandled exception for {Method} {Path}", httpContext.Request.Method, httpContext.Request.Path.Value);
                problem = new() { Status = StatusCodes.Status500InternalServerError, Title = "An unexpected error occurred" };
                break;
        }

        if (httpContext.Request.Headers["X-Request-ID"].FirstOrDefault() is { Length: > 0 } requestId)
        {
            problem.Extensions["request_id"] = requestId;
        }

        httpContext.Response.StatusCode = problem.Status!.Value;
        await httpContext.Response.WriteAsJsonAsync(problem, options: null, contentType: "application/problem+json", cancellationToken);
        return true;
    }
}
