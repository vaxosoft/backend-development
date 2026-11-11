using System.Text;
using Microsoft.Extensions.Time.Testing;
using Microsoft.IdentityModel.JsonWebTokens;
using Microsoft.IdentityModel.Tokens;

namespace M11.Exercises;

public class Ex2_TokenServiceTests
{
    private const string Secret = "unit-test-secret-long-enough-for-hs256-and-hs384-too";

    private static readonly TokenOptions Options = new("tracker", "tracker-api", Secret, TimeSpan.FromMinutes(30));

    private readonly FakeTimeProvider _clock = new(new DateTimeOffset(2030, 1, 1, 0, 0, 0, TimeSpan.Zero));

    private Ex2_TokenService Service(TokenOptions? options = null) => new(options ?? Options, _clock);

    [Fact]
    public void IssuedToken_CarriesSubjectAndRoleClaims()
    {
        var token = new JsonWebToken(Service().Issue("42", "admin"));

        Assert.Equal("42", token.Subject);
        Assert.Equal("admin", token.GetClaim("role").Value);
        Assert.Equal("HS256", token.Alg);
    }

    [Fact]
    public void Validate_RoundTripsAFreshToken()
    {
        var service = Service();

        Assert.Equal(new AuthenticatedUser("42", "admin"), service.Validate(service.Issue("42", "admin")));
    }

    [Fact]
    public void Validate_HonoursTheInjectedClockForExpiry()
    {
        var service = Service();
        var token = service.Issue("7", "user");

        _clock.Advance(TimeSpan.FromMinutes(29));
        Assert.NotNull(service.Validate(token));

        _clock.Advance(TimeSpan.FromMinutes(2));
        Assert.Null(service.Validate(token));
    }

    [Fact]
    public void Validate_RejectsTokensSignedWithAnotherSecret()
    {
        var other = Options with { Secret = "a-completely-different-32-byte-secret!!" };

        Assert.Null(Service().Validate(Service(other).Issue("1", "admin")));
    }

    [Theory]
    [InlineData("other-issuer", "tracker-api")]
    [InlineData("tracker", "other-audience")]
    public void Validate_RejectsWrongIssuerOrAudience(string issuer, string audience)
    {
        var foreign = Options with { Issuer = issuer, Audience = audience };

        Assert.Null(Service().Validate(Service(foreign).Issue("1", "user")));
    }

    [Fact]
    public void Validate_RejectsTamperedPayload()
    {
        var service = Service();
        var parts = service.Issue("1", "user").Split('.');
        var forged = Base64UrlEncoder.Encode("""{"sub":"1","role":"admin"}""");

        Assert.Null(service.Validate($"{parts[0]}.{forged}.{parts[2]}"));
    }

    [Fact]
    public void Validate_RejectsGarbage()
    {
        var service = Service();

        Assert.Null(service.Validate(""));
        Assert.Null(service.Validate("not-a-jwt"));
        Assert.Null(service.Validate("a.b.c"));
    }

    [Fact]
    public void Validate_RejectsAlgNoneTokens()
    {
        var header = Base64UrlEncoder.Encode("""{"alg":"none","typ":"JWT"}""");
        var exp = _clock.GetUtcNow().AddHours(1).ToUnixTimeSeconds();
        var payload = Base64UrlEncoder.Encode(
            $$"""{"sub":"1","role":"admin","iss":"tracker","aud":"tracker-api","exp":{{exp}}}""");

        Assert.Null(Service().Validate($"{header}.{payload}."));
    }

    [Fact]
    public void Validate_RejectsOtherHmacAlgorithmsEvenWithTheRightSecret()
    {
        var token = new JsonWebTokenHandler().CreateToken(new SecurityTokenDescriptor
        {
            Issuer = "tracker",
            Audience = "tracker-api",
            Expires = _clock.GetUtcNow().AddHours(1).UtcDateTime,
            Claims = new Dictionary<string, object> { ["sub"] = "1", ["role"] = "admin" },
            SigningCredentials = new SigningCredentials(
                new SymmetricSecurityKey(Encoding.UTF8.GetBytes(Secret)), SecurityAlgorithms.HmacSha384),
        });

        Assert.Null(Service().Validate(token));
    }

    [Fact]
    public void Constructor_RejectsShortSecrets() =>
        Assert.Throws<ArgumentException>(() => Service(Options with { Secret = "too-short" }));
}
