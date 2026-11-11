namespace P5.UrlShortener;

public static class AuthEndpoints
{
    /// <summary>
    /// <c>POST /auth/register</c>: body <c>{"email","password"}</c>; email must contain <c>@</c>, password at least 8 characters
    /// (else 422 validation problem); duplicate email -> 409; success -> 201 <see cref="RegisteredUser"/>.
    /// Emails are case-insensitive (store lower case).
    /// <c>POST /auth/login</c>: 200 <see cref="TokenResponse"/>; unknown user and wrong password -> the same 401 problem.
    /// </summary>
    public static void MapAuth(this IEndpointRouteBuilder app)
    {
        var group = app.MapGroup("/auth");
        group.MapPost("/register", Register);
        group.MapPost("/login", Login);
    }

    private static Task<IResult> Register(Credentials body, UserStore users, PasswordHasher hasher, CancellationToken cancellationToken)
    {
        // TODO(P5): validate, hash, store.
        _ = (body, users, hasher, cancellationToken);
        throw new NotImplementedException("TODO(P5): register");
    }

    private static Task<IResult> Login(Credentials body, UserStore users, PasswordHasher hasher, TokenService tokens, CancellationToken cancellationToken)
    {
        // TODO(P5): find the user, verify the password, issue a token.
        _ = (body, users, hasher, tokens, cancellationToken);
        throw new NotImplementedException("TODO(P5): login");
    }
}
