#pragma warning disable CS9113 // starter: your implementation will read these parameters

namespace M11.Exercises;

public sealed record TokenOptions(string Issuer, string Audience, string Secret, TimeSpan Lifetime);

public sealed record AuthenticatedUser(string UserId, string Role);

/// <summary>
/// Issues and validates HS256 JWTs. The <c>sub</c> claim carries the user id and <c>role</c> the role.
/// All time decisions come from the injected <see cref="TimeProvider"/>.
/// </summary>
public sealed class Ex2_TokenService(TokenOptions options, TimeProvider time)
{
    public string Issue(string userId, string role) => throw new NotImplementedException();

    /// <summary>Returns null for any invalid token: bad signature, expired, wrong issuer/audience/algorithm, garbage.</summary>
    public AuthenticatedUser? Validate(string token) => throw new NotImplementedException();
}
