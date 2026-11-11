using System.Security.Claims;

namespace P5.UrlShortener;

public static class LinkEndpoints
{
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
    private static Task<IResult> Create(CreateLinkRequest body, ClaimsPrincipal user, LinkStore links, CancellationToken cancellationToken)
    {
        // TODO(P5)
        _ = (body, user, links, cancellationToken);
        throw new NotImplementedException("TODO(P5): create link");
    }

    /// <summary>200 <see cref="LinkList"/> with only the caller's links.</summary>
    private static Task<IResult> List(ClaimsPrincipal user, LinkStore links, CancellationToken cancellationToken)
    {
        // TODO(P5)
        _ = (user, links, cancellationToken);
        throw new NotImplementedException("TODO(P5): list links");
    }

    /// <summary>204 for the owner, 403 for another user, 404 for an unknown slug.</summary>
    private static Task<IResult> Delete(string slug, ClaimsPrincipal user, LinkStore links, CancellationToken cancellationToken)
    {
        // TODO(P5)
        _ = (slug, user, links, cancellationToken);
        throw new NotImplementedException("TODO(P5): delete link");
    }

    /// <summary>302 with <c>Location</c> and the hit counter incremented; 404 for an unknown slug.</summary>
    private static Task<IResult> Redirect(string slug, LinkStore links, CancellationToken cancellationToken)
    {
        // TODO(P5)
        _ = (slug, links, cancellationToken);
        throw new NotImplementedException("TODO(P5): redirect");
    }
}
