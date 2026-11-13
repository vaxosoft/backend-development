"""Comprehensions, generators, closures and itertools."""

from collections import Counter, defaultdict
from collections.abc import Callable, Iterator
from functools import cache
from itertools import batched, islice


def main() -> None:
    titles = ["write docs", "Fix bug", "write tests", "Ship"]

    # Comprehensions build a new collection in one expression.
    print([t.title() for t in titles if " " in t])
    print({t: len(t) for t in titles})
    print(sorted({t[0].lower() for t in titles}))

    # Counting and grouping.
    print(Counter(t.split()[0].lower() for t in titles))
    groups: defaultdict[str, list[str]] = defaultdict(list)
    for t in titles:
        groups[t[0].lower()].append(t)
    print(dict(groups))

    # Sorting with a key function; sorted() returns a new list.
    print(sorted(titles, key=str.lower))
    print(sorted(titles, key=len, reverse=True))

    # Unpacking and enumerate/zip.
    first, *rest = titles
    print(first, "|", rest)
    for index, (title, size) in enumerate(zip(titles, map(len, titles), strict=True), start=1):
        print(index, title, size)

    # Generators are lazy: nothing runs until you iterate.
    evens = (n for n in naturals() if n % 2 == 0)
    print(list(islice(evens, 5)))
    print([list(chunk) for chunk in batched(range(7), 3, strict=False)])

    # Closures capture variables, not values.
    late: list[Callable[[], int]] = [lambda: i for i in range(3)]  # noqa: B023
    print("late binding:", [f() for f in late])
    bound = [make_const(i) for i in range(3)]
    print("early binding:", [f() for f in bound])

    # Modifying a list while iterating skips items; build a new one instead.
    numbers = [1, 2, 2, 3]
    for n in numbers:
        if n == 2:
            numbers.remove(n)
    print("after in-place removal:", numbers)
    print("filtered copy:", [n for n in [1, 2, 2, 3] if n != 2])

    print("fib(50) =", fib(50))


def naturals() -> Iterator[int]:
    n = 0
    while True:
        yield n
        n += 1


def make_const(value: int) -> Callable[[], int]:
    return lambda: value


@cache
def fib(n: int) -> int:
    return n if n < 2 else fib(n - 1) + fib(n - 2)


if __name__ == "__main__":
    main()
