"""Everything is an object: names are references, equality is not identity."""

import copy


def main() -> None:
    # `==` compares values, `is` compares identity.
    a = [1, 2, 3]
    b = [1, 2, 3]
    c = a
    print("a == b:", a == b)
    print("a is b:", a is b)
    print("a is c:", a is c)

    # Mutating through one name is visible through every alias.
    c.append(4)
    print("a after c.append(4):", a)

    # Immutable values (int, str, tuple) cannot change; "mutation" rebinds the name.
    s = "task"
    t = s
    s += "s"
    print("s, t:", s, t)

    # Shallow vs deep copy matters for nested data.
    original: dict[str, list[str]] = {"tags": ["docs"]}
    shallow = dict(original)
    deep = copy.deepcopy(original)
    original["tags"].append("p1")
    print("shallow tags:", shallow["tags"])
    print("deep tags:", deep["tags"])

    # Truthiness: empty containers, 0, "" and None are falsy.
    values: list[object] = [0, "", [], {}, None, "0", [0]]
    for value in values:
        print(f"bool({value!r}) = {bool(value)}")

    # Test for None with `is`, never with truthiness (0 and "" are valid values).
    limit: int | None = 0
    print("limit given:", limit is not None)

    # Mutable default arguments are evaluated once, at definition time.
    print("buggy:", add_tag_buggy("a"), add_tag_buggy("b"))
    print("fixed:", add_tag("a"), add_tag("b"))


def add_tag_buggy(tag: str, tags: list[str] = []) -> list[str]:  # noqa: B006
    tags.append(tag)
    return tags


def add_tag(tag: str, tags: list[str] | None = None) -> list[str]:
    result = [] if tags is None else list(tags)
    result.append(tag)
    return result


if __name__ == "__main__":
    main()
