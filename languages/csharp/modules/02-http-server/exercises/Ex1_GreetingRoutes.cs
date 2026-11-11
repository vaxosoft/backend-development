namespace M02.Exercises;

/// <summary>Routes that practice route, query and header binding.</summary>
public static class GreetingRoutes
{
    /// <summary>
    /// GET /hello             -> 200 text/plain "Hello, world!"
    /// GET /hello/{name}      -> 200 text/plain "Hello, {name}!"
    /// GET /add?a=1&amp;b=2   -> 200 JSON {"sum":3}; a missing or non-integer a/b -> 400
    /// GET /headers/{name}    -> 200 text/plain with the value of that request header; absent -> 404
    /// </summary>
    public static void Map(IEndpointRouteBuilder routes)
    {
        throw new NotImplementedException();
    }
}
