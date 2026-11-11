using Microsoft.AspNetCore.Authentication.JwtBearer;
using Microsoft.Extensions.Options;
using Microsoft.IdentityModel.Tokens;
using P5.UrlShortener;

var builder = WebApplication.CreateBuilder(args);

// Options are bound lazily so tests can override configuration before the host is built.
builder.Services.AddOptions<ShortenerOptions>()
    .BindConfiguration(ShortenerOptions.SectionName)
    .Validate(o => o.JwtSecret.Length >= 32, "Shortener:JwtSecret must be set (Shortener__JwtSecret) and be at least 32 characters.")
    .Validate(o => o.RateLimitBurst > 0 && o.RateLimitRefillPerSecond > 0, "rate limit burst and refill must be positive.")
    .ValidateOnStart();

builder.Services.AddSingleton(TimeProvider.System);
builder.Services.AddSingleton<ILogLineSink, ConsoleLogLineSink>();
builder.Services.AddSingleton<Database>();
builder.Services.AddSingleton<UserStore>();
builder.Services.AddSingleton<LinkStore>();
builder.Services.AddSingleton<PasswordHasher>();
builder.Services.AddSingleton<TokenService>();
builder.Services.AddProblemDetails();
builder.Services.AddLinkRateLimiting();

builder.Services.AddAuthentication(JwtBearerDefaults.AuthenticationScheme).AddJwtBearer();
builder.Services.AddOptions<JwtBearerOptions>(JwtBearerDefaults.AuthenticationScheme)
    .Configure<IOptions<ShortenerOptions>, TimeProvider>((jwt, shortener, time) =>
    {
        jwt.MapInboundClaims = false;
        jwt.TokenValidationParameters = new TokenValidationParameters
        {
            ValidateIssuer = true,
            ValidIssuer = ShortenerOptions.Issuer,
            ValidateAudience = true,
            ValidAudience = ShortenerOptions.Audience,
            ValidateLifetime = true,
            ValidateIssuerSigningKey = true,
            IssuerSigningKey = shortener.Value.SigningKey,
            ValidAlgorithms = [SecurityAlgorithms.HmacSha256],
            ClockSkew = TimeSpan.Zero,
            // JwtBearer has no TimeProvider hook; validating the lifetime ourselves makes expiry testable.
            LifetimeValidator = (notBefore, expires, _, _) =>
            {
                var now = time.GetUtcNow().UtcDateTime;
                return (notBefore is null || notBefore <= now) && (expires is null || expires > now);
            },
        };
    });
builder.Services.AddAuthorization();

var app = builder.Build();

app.UseMiddleware<RequestLoggingMiddleware>();
app.UseExceptionHandler();
app.UseAuthentication();
app.UseAuthorization();
app.UseRateLimiter();

app.MapAuth();
app.MapLinks();

// ADDR (host:port) controls the listening address. Ctrl-C / SIGTERM stop the host gracefully.
app.Run($"http://{Environment.GetEnvironmentVariable("ADDR") ?? "127.0.0.1:8080"}");

public partial class Program;
