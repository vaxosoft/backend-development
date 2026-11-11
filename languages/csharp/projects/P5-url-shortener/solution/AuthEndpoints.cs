namespace P5.UrlShortener;

public static class AuthEndpoints
{
    private const int MinPasswordLength = 8;

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

    private static async Task<IResult> Register(Credentials body, UserStore users, PasswordHasher hasher, CancellationToken cancellationToken)
    {
        var errors = new Dictionary<string, string[]>();
        var email = Normalize(body.Email);
        if (!email.Contains('@'))
        {
            errors["email"] = ["must be a valid email address"];
        }

        if (body.Password is null || body.Password.Length < MinPasswordLength)
        {
            errors["password"] = [$"must be at least {MinPasswordLength} characters"];
        }

        if (errors.Count > 0)
        {
            return Results.ValidationProblem(errors, statusCode: StatusCodes.Status422UnprocessableEntity);
        }

        var id = await users.CreateAsync(email, hasher.Hash(body.Password!), cancellationToken);
        return id is { } newId
            ? Results.Json(new RegisteredUser(newId, email), statusCode: StatusCodes.Status201Created)
            : Results.Problem(statusCode: StatusCodes.Status409Conflict, title: "Conflict", detail: "email already registered");
    }

    private static async Task<IResult> Login(Credentials body, UserStore users, PasswordHasher hasher, TokenService tokens, CancellationToken cancellationToken)
    {
        var password = body.Password ?? "";
        var user = await users.FindByEmailAsync(Normalize(body.Email), cancellationToken);
        if (user is null)
        {
            // Spend the same CPU time as a real check so response timing does not reveal which emails exist.
            hasher.Hash(password);
        }

        return user is not null && hasher.Verify(password, user.PasswordHash)
            ? Results.Ok(new TokenResponse(tokens.Issue(user.Id, user.Email)))
            : Results.Problem(statusCode: StatusCodes.Status401Unauthorized, title: "Unauthorized", detail: "invalid email or password");
    }

    private static string Normalize(string? email) => (email ?? "").Trim().ToLowerInvariant();
}
