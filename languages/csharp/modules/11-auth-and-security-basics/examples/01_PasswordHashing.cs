using System.Security.Cryptography;
using System.Text;

namespace M11.Examples;

public static class PasswordHashing
{
    private const int SaltSize = 16;
    private const int HashSize = 32;
    private const int Iterations = 600_000;

    public static string Hash(string password)
    {
        var salt = RandomNumberGenerator.GetBytes(SaltSize);
        var hash = Rfc2898DeriveBytes.Pbkdf2(password, salt, Iterations, HashAlgorithmName.SHA256, HashSize);
        return $"pbkdf2-sha256${Iterations}${Convert.ToBase64String(salt)}${Convert.ToBase64String(hash)}";
    }

    public static bool Verify(string password, string stored)
    {
        var parts = stored.Split('$');
        if (parts is not ["pbkdf2-sha256", var iterText, var saltText, var hashText]
            || !int.TryParse(iterText, out var iterations))
        {
            return false;
        }

        var expected = Convert.FromBase64String(hashText);
        var actual = Rfc2898DeriveBytes.Pbkdf2(
            password, Convert.FromBase64String(saltText), iterations, HashAlgorithmName.SHA256, expected.Length);
        return CryptographicOperations.FixedTimeEquals(actual, expected);
    }

    public static Task<int> RunAsync(string[] args)
    {
        var first = Hash("correct horse battery staple");
        var second = Hash("correct horse battery staple");

        Console.WriteLine($"format: {first.Split('$')[0]}${first.Split('$')[1]}$<salt>$<hash>");
        Console.WriteLine($"same password, different stored values: {first != second}");
        Console.WriteLine($"verify correct password: {Verify("correct horse battery staple", first)}");
        Console.WriteLine($"verify wrong password: {Verify("Tr0ub4dor&3", first)}");
        Console.WriteLine($"verify garbage record: {Verify("x", "not-a-hash")}");

        // PBKDF2-HMAC-SHA256 known-answer test (password="password", salt="salt", 1 iteration)
        var kat = Rfc2898DeriveBytes.Pbkdf2(
            Encoding.UTF8.GetBytes("password"), Encoding.UTF8.GetBytes("salt"), 1, HashAlgorithmName.SHA256, 32);
        Console.WriteLine($"known answer: {Convert.ToHexStringLower(kat)}");
        return Task.FromResult(0);
    }
}
