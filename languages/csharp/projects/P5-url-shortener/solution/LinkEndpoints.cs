using System.Security.Claims;
using System.Security.Cryptography;
using System.Text.RegularExpressions;

namespace P5.UrlShortener;

public static partial class LinkEndpoints
{
    private const string SlugAlphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789";
    private const int GeneratedSlugLength = 7;
    private const int MaxGenerateAttempts = 5;
    private static readonly string[] ReservedSlugs = ["links", "auth"];

    [GeneratedRegex("^[A-Za-z0-9_-]{3,32}$")]
    private static partial Regex SlugPattern();

    /// <summary>
    /// <c>POST /links</c> (auth, rate limited), <c>GET /links</c> (auth), <c>DELETE /links/{slug}</c> (auth, owner only)
    /// and the public <c>GET /{slug}</c> redirect. Literal routes such as <c>/links</c> win over <c>/{slug}</c>.
    /// </summary>
    public static void MapLinks(this IEndpointRouteBuilder app)
    {
        var group = app.MapGroup("/links").RequireAuthorization();
        group.MapPost("", Create).RequireRateLimiting(RateLimiting.CreateLinkPolicy);
        group.MapGet("", List);
        group.MapDelete("/{slug}", Delete);

        app.MapGet("/{slug}", Redirect);
    }

    /// <summary>
    /// Body <c>{"url","slug"?}</c>. url: absolute http/https -> else 422. slug: optional; when given 3-32 characters of
    /// <c>[A-Za-z0-9_-]</c> and not a reserved word (<c>links</c>, <c>auth</c>) -> else 422; when omitted generate 7
    /// random <c>[A-Za-z0-9]</c> characters (retry on collision). Taken slug -> 409. Success -> 201 <see cref="LinkResponse"/>.
    /// </summary>
    private static async Task<IResult> Create(CreateLinkRequest body, ClaimsPrincipal user, LinkStore links, CancellationToken cancellationToken)
    {
        var errors = new Dictionary<string, string[]>();
        if (!Uri.TryCreate(body.Url, UriKind.Absolute, out var target) || target.Scheme is not ("http" or "https") || target.Host.Length == 0)
        {
            errors["url"] = ["must be an absolute http or https URL"];
        }

        if (body.Slug is not null && (!SlugPattern().IsMatch(body.Slug) || ReservedSlugs.Contains(body.Slug, StringComparer.OrdinalIgnoreCase)))
        {
            errors["slug"] = ["must be 3-32 characters of A-Z, a-z, 0-9, _ or - and not a reserved word"];
        }

        if (errors.Count > 0)
        {
            return Results.ValidationProblem(errors, statusCode: StatusCodes.Status422UnprocessableEntity);
        }

        var ownerId = user.GetUserId()!.Value;
        var url = body.Url!;
        if (body.Slug is not null)
        {
            return await links.CreateAsync(body.Slug, url, ownerId, cancellationToken)
                ? Created(body.Slug, url)
                : Conflict();
        }

        for (var attempt = 0; attempt < MaxGenerateAttempts; attempt++)
        {
            var slug = RandomNumberGenerator.GetString(SlugAlphabet, GeneratedSlugLength);
            if (await links.CreateAsync(slug, url, ownerId, cancellationToken))
            {
                return Created(slug, url);
            }
        }

        return Conflict();
    }

    /// <summary>200 <see cref="LinkList"/> with only the caller's links.</summary>
    private static async Task<IResult> List(ClaimsPrincipal user, LinkStore links, CancellationToken cancellationToken)
    {
        var items = await links.ListByOwnerAsync(user.GetUserId()!.Value, cancellationToken);
        return Results.Ok(new LinkList(items, items.Count));
    }

    /// <summary>204 for the owner, 403 for another user, 404 for an unknown slug.</summary>
    private static async Task<IResult> Delete(string slug, ClaimsPrincipal user, LinkStore links, CancellationToken cancellationToken)
    {
        var link = await links.FindAsync(slug, cancellationToken);
        if (link is null)
        {
            return Results.Problem(statusCode: StatusCodes.Status404NotFound, title: "Not Found");
        }

        if (link.OwnerId != user.GetUserId())
        {
            return Results.Problem(statusCode: StatusCodes.Status403Forbidden, title: "Forbidden", detail: "only the owner can delete a link");
        }

        await links.DeleteAsync(slug, cancellationToken);
        return Results.NoContent();
    }

    /// <summary>302 with <c>Location</c> and the hit counter incremented; 404 for an unknown slug.</summary>
    private static async Task<IResult> Redirect(string slug, LinkStore links, CancellationToken cancellationToken)
    {
        var url = await links.HitAsync(slug, cancellationToken);
        return url is null
            ? Results.Problem(statusCode: StatusCodes.Status404NotFound, title: "Not Found")
            : Results.Redirect(url);
    }

    private static IResult Created(string slug, string url) =>
        Results.Json(new LinkResponse(slug, url), statusCode: StatusCodes.Status201Created);

    private static IResult Conflict() =>
        Results.Problem(statusCode: StatusCodes.Status409Conflict, title: "Conflict", detail: "slug is already taken");
}
