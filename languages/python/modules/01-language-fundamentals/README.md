# 01. Language Fundamentals in Python

> Estimated time: 1.5-2h | Difficulty: Beginner | Prerequisites: none (any programming experience)

## Learning Outcomes
After this module you can:
- [ ] Explain that Python names are references to objects, and predict what is shared, copied or rebound
- [ ] Model data with `dataclass`, `Enum` and `Protocol`, and prefer composition to inheritance
- [ ] Define a small exception hierarchy, chain causes with `raise ... from`, and manage resources with `with`
- [ ] Use comprehensions, generators, closures and `itertools` without falling into the classic traps
- [ ] Annotate code so that `mypy --strict`, `ruff` and `pytest` all pass

## 1. Concept Overview
Every backend language needs the same toolbox: types and control flow, a way to group data and behaviour, a way to express failure, collections and iteration, and a project layout with tooling. This module covers that toolbox in Python so that the HTTP modules can focus on the web.

Two ideas matter most in Python. First, *everything is an object* and variables are just names bound to objects, so "copy or alias?" is a question you must always be able to answer. Second, behaviour is usually expressed through small protocols (iteration, context managers, `__eq__`, ...) rather than class hierarchies.

## 2. How Python Approaches It

| Need | Idiomatic Python | Avoid |
|------|------------------|-------|
| Plain data | `@dataclass(slots=True, frozen=True)` | hand-written `__init__`/`__repr__`, bare dicts everywhere |
| Fixed set of values | `enum.StrEnum` (wire value = member value) | string constants scattered around |
| Interfaces | `typing.Protocol` (structural, no inheritance needed) | deep base-class hierarchies |
| Errors | exceptions, one base class per app (`AppError`) | returning error codes, bare `except:` |
| Resources | `with` + `contextlib.contextmanager` | manual `try/finally` copies |
| Collections | `list`, `dict`, `set`, `tuple`, `collections.Counter/defaultdict` | mutating a list while iterating it |
| Lazy sequences | generator functions and expressions, `itertools` | building huge lists just to loop once |
| Types | built-in generics (`list[int]`), `X \| None`, checked with `mypy --strict` | `typing.List`, `Optional[...]`, untyped code |

Visibility is by convention: a leading underscore means private. Modules are files, packages are folders; a module's top-level code runs once on first import, so guard scripts with `if __name__ == "__main__":`.

**Memory model:** CPython frees objects by reference counting plus a cycle collector. There are no pointers, but every variable is a reference, so *mutable* objects (`list`, `dict`, `set`, dataclasses without `frozen`) are shared between aliases while *immutable* ones (`int`, `str`, `tuple`, `frozenset`) are not.

**Tooling** (all commands from `languages/python/` with the virtual environment active):

| Purpose | Command |
|---------|---------|
| Run | `python modules/01-language-fundamentals/examples/01_values_and_references.py` |
| Test | `pytest modules/01-language-fundamentals/exercises/ex1_pitfalls` |
| Format | `ruff format modules/01-language-fundamentals` |
| Lint | `ruff check modules/01-language-fundamentals` |
| Types | `mypy modules/01-language-fundamentals/examples` |
| Debug | `breakpoint()` (pdb), `python -X dev` for extra runtime checks |

## 3. Guided Examples

### 3.1 Values, references and mutability
- **Goal:** know when two names share one object.
- **Run:** `python modules/01-language-fundamentals/examples/01_values_and_references.py`
- **Code:** [examples/01_values_and_references.py](examples/01_values_and_references.py)
- **Walkthrough:** `==` compares values and `is` compares identity. Appending through alias `c` changes `a`. `s += "s"` rebinds `s` to a new string (strings are immutable), so `t` is unchanged. `dict(original)` is a shallow copy, so the inner list is still shared; `copy.deepcopy` isolates it. Default argument values are evaluated once, which is why `add_tag_buggy` leaks state between calls.

```python
def add_tag(tag: str, tags: list[str] | None = None) -> list[str]:
    result = [] if tags is None else list(tags)
    result.append(tag)
    return result
```

- **Expected output (excerpt):**

  ```
  a == b: True
  a is b: False
  a is c: True
  a after c.append(4): [1, 2, 3, 4]
  s, t: tasks task
  shallow tags: ['docs', 'p1']
  deep tags: ['docs']
  bool(0) = False
  bool('') = False
  bool([]) = False
  bool({}) = False
  bool(None) = False
  bool('0') = True
  bool([0]) = True
  limit given: True
  buggy: ['a', 'b'] ['a', 'b']
  fixed: ['a'] ['b']
  ```

### 3.2 Data modeling: dataclass, Enum, Protocol, composition
- **Goal:** model a domain without boilerplate or inheritance.
- **Run:** `python modules/01-language-fundamentals/examples/02_data_modeling.py`
- **Code:** [examples/02_data_modeling.py](examples/02_data_modeling.py)
- **Walkthrough:** `Task` is frozen, so changes go through `dataclasses.replace`, which returns a new object. `Status("finished")` raises `ValueError`, which is how enum values from the wire are validated. `Notifier` is a `Protocol`: `ConsoleNotifier` and `RecordingNotifier` never inherit from it, they just have a matching `notify`. `TaskService` *has a* notifier (composition), so tests can inject a recording one.

```python
class Notifier(Protocol):
    def notify(self, message: str) -> None: ...


class TaskService:
    def __init__(self, notifier: Notifier) -> None:
        self._notifier = notifier
```

- **Expected output:**

  ```
  Task(id=1, title='Write docs', status=<Status.TODO: 'todo'>, tags=('docs', 'p1'))
  equal copy: True
  frozen: FrozenInstanceError
  status from wire value: IN_PROGRESS
  bad status: 'finished' is not a valid Status
  notify: task 1 is done
  original untouched: todo | new: done
  recorded: ['task 1 is done']
  counter: Counter(seen=['x']) | fresh: Counter(seen=[])
  ```

### 3.3 Errors and context managers
- **Goal:** raise, chain and clean up properly.
- **Run:** `python modules/01-language-fundamentals/examples/03_errors_and_context_managers.py`
- **Code:** [examples/03_errors_and_context_managers.py](examples/03_errors_and_context_managers.py)
- **Walkthrough:** `raise ValidationError(...) from err` keeps the original `ValueError` as `__cause__`; `from None` hides a noisy cause (used for `KeyError` to `NotFoundError`). `try/else/finally`: `else` runs only on success, `finally` always. `logged` is a generator-based context manager: code before `yield` is setup, `finally` is teardown, and the exception still propagates. `except*` handles an `ExceptionGroup` (you will meet it with `asyncio.TaskGroup`).

```python
@contextmanager
def logged(step: str) -> Iterator[None]:
    print(f"[{step}] start")
    try:
        yield
    finally:
        print(f"[{step}] end")
```

- **Expected output:**

  ```
  found: Write docs
  not found: task 2 not found -> task 2
  validation: {'id': 'must be an integer'} | cause: ValueError("invalid literal for int() with base 10: 'x'")
  else branch: Write docs
  finally always runs
  [lookup] start
  [lookup] failed: task 99 not found
  [lookup] end
  caught outside: NotFoundError
  group validation: ['ValidationError']
  group not found: ['task 7 not found']
  ```

### 3.4 Collections, iteration, closures
- **Goal:** write expressive loops and avoid the closure and mutation traps.
- **Run:** `python modules/01-language-fundamentals/examples/04_collections_and_iteration.py`
- **Code:** [examples/04_collections_and_iteration.py](examples/04_collections_and_iteration.py)
- **Walkthrough:** comprehensions for `list`/`dict`/`set`; `Counter` and `defaultdict` for tallies; `sorted(key=...)`; `zip(..., strict=True)` catches length mismatches; `*rest` unpacking. A generator (`naturals`) is lazy, so `islice` can take five items from an infinite stream. Closures capture the *variable* `i`, not its value: every lambda in `late` sees the final `2`; a factory function (`make_const`) binds each value.

```python
late = [lambda: i for i in range(3)]  # [2, 2, 2]
bound = [make_const(i) for i in range(3)]  # [0, 1, 2]
```

- **Expected output (excerpt):**

  ```
  ['Write Docs', 'Fix Bug', 'Write Tests']
  {'write docs': 10, 'Fix bug': 7, 'write tests': 11, 'Ship': 4}
  ['f', 's', 'w']
  Counter({'write': 2, 'fix': 1, 'ship': 1})
  [0, 2, 4, 6, 8]
  [[0, 1, 2], [3, 4, 5], [6]]
  late binding: [2, 2, 2]
  early binding: [0, 1, 2]
  after in-place removal: [1, 2, 3]
  filtered copy: [1, 3]
  fib(50) = 12586269025
  ```

### 3.5 Task Tracker domain in plain Python
- **Goal:** put it together: model, validation, repository protocol, queries.
- **Run:** `python modules/01-language-fundamentals/examples/05_task_domain.py`
- **Code:** [examples/05_task_domain.py](examples/05_task_domain.py)
- **Walkthrough:** `validate` collects *every* problem into one `ValidationError` (the same shape the REST modules return as `fields`). `TaskRepository` is a `Protocol`; `InMemoryTaskRepository` satisfies it structurally, and later modules swap in SQLite. `by_status` returns a lazy generator. `:=` (walrus) binds and tests in one step.

- **Expected output:**

  ```
  1 'Write docs'   todo        due=- tags=['docs', 'p1']
  2 'Fix bug'      in_progress due=2030-01-31 tags=['bug', 'p1']
  3 'Ship it'      todo        due=- tags=[]
  in progress: ['Fix bug']
  tag counts: {'bug': 1, 'docs': 1, 'p1': 2}
  missing: None
  rejected: {'title': 'must be 1-200 characters', 'tags': 'each tag must be 1-30 lowercase characters'}
  ```

## 4. Common Pitfalls

| Mistake | Why it happens | Fix |
|---------|----------------|-----|
| `def f(x, items=[])` leaks state between calls | defaults are evaluated once when `def` runs | default to `None` and create the list inside, or use `field(default_factory=list)` in dataclasses |
| `[lambda: i for i in range(3)]` all return `2` | closures capture the variable, not its value at creation | bind through a factory function or a default argument (`lambda i=i: i`) |
| `if x == None` or `if not limit` when `0` is valid | `==` can be overridden and `0`/`""` are falsy | `x is None` / `x is not None` |
| `for n in nums: nums.remove(n)` skips items | the list shifts under the iterator | build a new list with a comprehension |
| `b = dict(a)` still shares nested lists | `dict()`, `list()`, `.copy()` are shallow | `copy.deepcopy`, or use immutable data (`tuple`, frozen dataclass) |
| `except Exception: pass` hides bugs | it is the shortest way to "handle" an error | catch the narrowest type, log or re-raise, never a bare `except:` |
| `raise NewError()` inside `except` loses the root cause in your code | the cause is only implicit | `raise NewError(...) from err` |
| Mutating a `frozen` dataclass at runtime | frozen only blocks attribute assignment, not mutation of a contained list | store `tuple`s, not `list`s, in frozen dataclasses |

## 5. Coming From Other Languages
- **JavaScript/TypeScript:** type hints are like TS types but never enforced at runtime; run `mypy`. `None` is the only "nothing" (no `undefined`), `0`/`""`/`[]`/`{}` are falsy (including empty containers, unlike JS).
- **Java/C#:** no `new`, no access modifiers; dataclasses replace getters/setters and records. Protocols are interfaces you satisfy implicitly. Decorators are plain functions, not annotations.
- **Go:** exceptions instead of `(value, error)`; `Protocol` is the equivalent of an implicit interface; `with` plays the role of `defer`. Dict iteration order is insertion order (guaranteed).
- **Ruby:** closures and iteration feel familiar, but there are no blocks: pass a function or use a generator.

## 6. Exercises
Each exercise has starter code and tests in `exercises/`; reference solutions are in `solutions/` (try first!).

| # | Title | Difficulty | Skill practiced | Run tests |
|---|-------|-----------|-----------------|-----------|
| 1 | [Pitfalls](exercises/ex1_pitfalls/m01_pitfalls.py) | Easy | defaults, closures, aliasing, deep copy | `pytest modules/01-language-fundamentals/exercises/ex1_pitfalls` |
| 2 | [Task model](exercises/ex2_task_model/m01_task_model.py) | Medium | dataclass, `Enum`, collected validation | `pytest modules/01-language-fundamentals/exercises/ex2_task_model` |
| 3 | [Unit of work](exercises/ex3_unit_of_work/m01_unit_of_work.py) | Stretch | context manager, generator, decorator | `pytest modules/01-language-fundamentals/exercises/ex3_unit_of_work` |

**Exercise 1: Pitfalls**
- **Task:** implement `add_tag`, `make_multipliers`, `remove_all` and `merge_settings` so none of them mutates inputs or shares state.
- **Acceptance:** all tests in `test_m01_pitfalls.py` pass.
- <details><summary>Hint</summary>Return new objects; for the multipliers, create the lambda inside a helper function.</details>

**Exercise 2: Task model**
- **Task:** implement `Task.to_dict` and `parse_task`, which validates untrusted input and reports every invalid field at once.
- **Acceptance:** all tests in `test_m01_task_model.py` pass (note that `True` is not a valid `owner_id`).
- <details><summary>Hint</summary>Collect problems in a dict, <code>Status(value)</code> raises <code>ValueError</code> for bad values, and <code>isinstance(True, int)</code> is <code>True</code>.</details>

**Exercise 3: Unit of work**
- **Task:** make `KeyValueStore.transaction()` roll back on exceptions, `paginate` lazily chunk any iterable, and `retry` a typed decorator factory.
- **Acceptance:** all tests in `test_m01_unit_of_work.py` pass, including laziness on an infinite iterator and `__name__` preservation.
- <details><summary>Hint</summary>Snapshot with <code>copy.deepcopy</code> before <code>yield</code>; <code>itertools.batched</code> does the chunking; <code>functools.wraps</code> keeps metadata.</details>

## 7. Recap
- Names are references: know what is shared (mutable) and what is copied or rebound (immutable).
- `dataclass` + `Enum` + `Protocol` cover most modelling needs without inheritance.
- Exceptions form a small hierarchy; chain causes and clean up with `with`.
- Comprehensions and generators are the default loop tools; closures capture variables.
- Use `X | None` and `is None`; run `ruff` and `mypy --strict` on everything.

When to reach for what: plain data -> frozen dataclass; closed set of values -> `StrEnum`; swappable dependency -> `Protocol`; resource with cleanup -> context manager.

## 8. Self-check Questions
1. Why does `a = [[]] * 3; a[0].append(1)` change all three inner lists, and how would you build independent ones?
2. When is a frozen dataclass *not* actually immutable?
3. What is the difference between `raise X from err` and `raise X from None`, and when would you use each?
4. Why can a generator process a 10 GB file with constant memory, while the equivalent list comprehension cannot?
5. Why is `Protocol` a better fit than an abstract base class for a repository interface used in tests?

## 9. Further Reading
- [The Python Tutorial](https://docs.python.org/3/tutorial/) and the [data model reference](https://docs.python.org/3/reference/datamodel.html)
- [`dataclasses`](https://docs.python.org/3/library/dataclasses.html), [`typing.Protocol`](https://docs.python.org/3/library/typing.html#typing.Protocol), [`contextlib`](https://docs.python.org/3/library/contextlib.html)
- [mypy documentation](https://mypy.readthedocs.io/) and [Ruff rules](https://docs.astral.sh/ruff/rules/)

---
**Next:** [02 HTTP Server](../02-http-server/README.md)
