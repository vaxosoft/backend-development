using System.Collections.Concurrent;
using System.Security.Claims;
using System.Threading.RateLimiting;
using Microsoft.AspNetCore.Authentication.JwtBearer;
using Microsoft.IdentityModel.JsonWebTokens;
using Microsoft.IdentityModel.Tokens;

namespace M11.Examples;

public static class TrackerAuthApi
{
    private sealed record Account(int Id, string Email, string PasswordHash, string Role);

    private sealed record Credentials(string? Email, string? Password);

    private sealed record TaskInput(string? Title);

    public static async Task<int> RunAsync(string[] args)
    {
        var addr = Environment.GetEnvironmentVariable("ADDR") ?? "127.0.0.1:8080";
        var secret = Environment.GetEnvironmentVariable("JWT_SECRET") ?? "dev-only-secret-change-me-32-bytes-min!";
        var adminEmail = Environment.GetEnvironmentVariable("ADMIN_EMAIL") ?? "admin@example.com";
        var key = new SymmetricSecurityKey(System.Text.Encoding.UTF8.GetBytes(secret));

        var builder = WebApplication.CreateBuilder(args);
        builder.WebHost.UseUrls($"http://{addr}");
        builder.Logging.SetMinimumLevel(LogLevel.Warning);
        builder.Services.AddProblemDetails();

        builder.Services.AddAuthentication(JwtBearerDefaults.AuthenticationScheme)
            .AddJwtBearer(o =>
            {
                o.MapInboundClaims = false;
                o.TokenValidationParameters = new TokenValidationParameters
                {
                    ValidIssuer = "tracker",
                    ValidAudience = "tracker-api",
                    IssuerSigningKey = key,
                    ValidAlgorithms = [SecurityAlgorithms.HmacSha256],
                    ClockSkew = TimeSpan.FromSeconds(5),
                    NameClaimType = "sub",
                    RoleClaimType = "role",
                };
            });
        builder.Services.AddAuthorizationBuilder()
            .AddPolicy("AdminOnly", p => p.RequireRole("admin"));

        builder.Services.AddRateLimiter(o =>
        {
            o.RejectionStatusCode = StatusCodes.Status429TooManyRequests;
            o.AddPolicy("create-task", http =>
                RateLimitPartition.GetTokenBucketLimiter(
                    http.User.Identity?.Name ?? "anonymous",
                    _ => new TokenBucketRateLimiterOptions
                    {
                        TokenLimit = 3,
                        TokensPerPeriod = 1,
                        ReplenishmentPeriod = TimeSpan.FromSeconds(30),
                        QueueLimit = 0,
                    }));
            o.OnRejected = async (context, ct) =>
            {
                if (context.Lease.TryGetMetadata(MetadataName.RetryAfter, out var after))
                {
                    context.HttpContext.Response.Headers.RetryAfter = ((int)after.TotalSeconds).ToString();
                }

                await context.HttpContext.Response.WriteAsJsonAsync(
                    new { title = "Too many requests", status = 429 }, ct);
            };
        });

        var app = builder.Build();
        app.UseAuthentication();
        app.UseAuthorization();
        app.UseRateLimiter();

        var accounts = new ConcurrentDictionary<string, Account>(StringComparer.OrdinalIgnoreCase);
        var nextId = 0;
        var tasks = new ConcurrentQueue<string>();

        app.MapPost("/auth/register", (Credentials body) =>
        {
            if (string.IsNullOrWhiteSpace(body.Email) || !body.Email.Contains('@') || body.Password is not { Length: >= 8 })
            {
                return Results.ValidationProblem(
                    new Dictionary<string, string[]> { ["credentials"] = ["email must contain @ and password must have 8+ characters"] },
                    statusCode: StatusCodes.Status422UnprocessableEntity);
            }

            if (accounts.ContainsKey(body.Email))
            {
                return Results.Problem("email already registered", statusCode: StatusCodes.Status409Conflict);
            }

            var role = body.Email.Equals(adminEmail, StringComparison.OrdinalIgnoreCase) ? "admin" : "user";
            var account = new Account(Interlocked.Increment(ref nextId), body.Email, PasswordHashing.Hash(body.Password), role);
            return accounts.TryAdd(body.Email, account)
                ? Results.Created($"/users/{account.Id}", new { id = account.Id, email = account.Email })
                : Results.Problem("email already registered", statusCode: StatusCodes.Status409Conflict);
        });

        app.MapPost("/auth/login", (Credentials body) =>
        {
            // Same answer for unknown user and wrong password: no account enumeration.
            if (body.Email is null || body.Password is null
                || !accounts.TryGetValue(body.Email, out var account)
                || !PasswordHashing.Verify(body.Password, account.PasswordHash))
            {
                return Results.Problem("invalid credentials", statusCode: StatusCodes.Status401Unauthorized);
            }

            var token = new JsonWebTokenHandler().CreateToken(new SecurityTokenDescriptor
            {
                Issuer = "tracker",
                Audience = "tracker-api",
                Expires = DateTime.UtcNow.AddHours(1),
                Subject = new ClaimsIdentity([new Claim("sub", account.Id.ToString()), new Claim("role", account.Role)]),
                SigningCredentials = new SigningCredentials(key, SecurityAlgorithms.HmacSha256),
            });
            return Results.Ok(new { token });
        });

        app.MapGet("/me", (ClaimsPrincipal user) => new { id = user.Identity!.Name, role = user.FindFirstValue("role") })
            .RequireAuthorization();

        app.MapGet("/admin/stats", () => new { users = accounts.Count, tasks = tasks.Count })
            .RequireAuthorization("AdminOnly");

        app.MapPost("/tasks", (TaskInput body) =>
        {
            if (string.IsNullOrWhiteSpace(body.Title))
            {
                return Results.ValidationProblem(
                    new Dictionary<string, string[]> { ["title"] = ["required"] },
                    statusCode: StatusCodes.Status422UnprocessableEntity);
            }

            tasks.Enqueue(body.Title);
            return Results.Created($"/tasks/{tasks.Count}", new { title = body.Title });
        }).RequireAuthorization().RequireRateLimiting("create-task");

        await app.RunAsync();
        return 0;
    }
}
