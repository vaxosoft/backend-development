using System.Security.Claims;

namespace P5.UrlShortener;

public static class ClaimsPrincipalExtensions
{
    /// <summary>The authenticated user's id (the JWT <c>sub</c> claim), or null for anonymous callers.</summary>
    public static long? GetUserId(this ClaimsPrincipal principal) =>
        long.TryParse(principal.FindFirstValue("sub"), out var id) ? id : null;
}
