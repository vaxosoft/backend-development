from m01_pitfalls import add_tag, make_multipliers, merge_settings, remove_all


def test_add_tag_returns_fresh_list_each_call() -> None:
    assert add_tag("a") == ["a"]
    assert add_tag("b") == ["b"]


def test_add_tag_does_not_mutate_argument() -> None:
    existing = ["x"]
    result = add_tag("y", existing)
    assert result == ["x", "y"]
    assert existing == ["x"]
    assert result is not existing


def test_multipliers_bind_each_value() -> None:
    funcs = make_multipliers(4)
    assert [f(10) for f in funcs] == [0, 10, 20, 30]


def test_multipliers_zero_count() -> None:
    assert make_multipliers(0) == []


def test_remove_all_handles_adjacent_duplicates() -> None:
    items = [1, 2, 2, 3, 2]
    assert remove_all(items, 2) == [1, 3]
    assert items == [1, 2, 2, 3, 2]


def test_merge_settings_is_recursive_and_override_wins() -> None:
    base = {"server": {"host": "localhost", "port": 8080}, "debug": False}
    override = {"server": {"port": 9000}, "debug": True, "name": "api"}
    assert merge_settings(base, override) == {
        "server": {"host": "localhost", "port": 9000},
        "debug": True,
        "name": "api",
    }


def test_merge_settings_does_not_alias_or_mutate() -> None:
    base = {"server": {"port": 8080}, "tags": ["a"]}
    override = {"extra": {"flags": ["x"]}}
    result = merge_settings(base, override)
    result["server"]["port"] = 1
    result["tags"].append("b")
    result["extra"]["flags"].append("y")
    assert base == {"server": {"port": 8080}, "tags": ["a"]}
    assert override == {"extra": {"flags": ["x"]}}
