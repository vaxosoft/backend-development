using System.ComponentModel.DataAnnotations;
using System.Text.Json;
using Microsoft.AspNetCore.Http.HttpResults;

namespace M03.Examples;

static class Problems
{
    const string UnprocessableType = "https://www.rfc-editor.org/rfc/rfc9110#section-15.5.21";

    /// <summary>
    /// One place that shapes every ProblemDetails: validation failures from AddValidation become
    /// 422 with snake_case field names (the framework default is 400 with property names).
    /// </summary>
    public static void Customize(ProblemDetailsContext context)
    {
        if (context.ProblemDetails is not HttpValidationProblemDetails validation)
        {
            return;
        }

        var renamed = validation.Errors.ToDictionary(e => JsonNamingPolicy.SnakeCaseLower.ConvertName(e.Key), e => e.Value);
        validation.Errors.Clear();
        foreach (var (field, messages) in renamed)
        {
            validation.Errors[field] = messages;
        }

        validation.Status = StatusCodes.Status422UnprocessableEntity;
        validation.Type = UnprocessableType;
        context.HttpContext.Response.StatusCode = StatusCodes.Status422UnprocessableEntity;
    }

    /// <summary>Manual validation for values that did not come through AddValidation (e.g. a merged PATCH).</summary>
    public static ProblemHttpResult? Validate(object model)
    {
        var results = new List<ValidationResult>();
        if (Validator.TryValidateObject(model, new ValidationContext(model), results, validateAllProperties: true))
        {
            return null;
        }

        var errors = results
            .SelectMany(r => r.MemberNames.DefaultIfEmpty(string.Empty), (r, member) => (member, message: r.ErrorMessage ?? "is invalid"))
            .GroupBy(e => JsonNamingPolicy.SnakeCaseLower.ConvertName(e.member), e => e.message)
            .ToDictionary(g => g.Key, g => g.ToArray());
        return TypedResults.Problem(new HttpValidationProblemDetails(errors)
        {
            Status = StatusCodes.Status422UnprocessableEntity,
            Type = UnprocessableType,
        });
    }

    public static ProblemHttpResult NotFound(string resource, int id) =>
        TypedResults.Problem(statusCode: StatusCodes.Status404NotFound, title: "Not Found", detail: $"{resource} {id} does not exist");

    public static ProblemHttpResult Conflict(string detail) =>
        TypedResults.Problem(statusCode: StatusCodes.Status409Conflict, title: "Conflict", detail: detail);

    public static ProblemHttpResult Invalid(string field, string message) =>
        TypedResults.Problem(new HttpValidationProblemDetails(new Dictionary<string, string[]> { [field] = [message] })
        {
            Status = StatusCodes.Status422UnprocessableEntity,
            Type = UnprocessableType,
        });
}
