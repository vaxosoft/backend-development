"""Diagnostics from the stdlib: tracemalloc, cProfile and faulthandler."""

import cProfile
import faulthandler
import pstats
import sys
import tracemalloc


def build_index(n: int) -> dict[int, str]:
    return {i: f"task-{i}" for i in range(n)}


def slow_search(items: list[int], wanted: int) -> bool:
    return any(item == wanted for item in items)  # O(n) per call


def many_searches() -> int:
    items = list(range(2_000))
    return sum(slow_search(items, n) for n in range(0, 4_000, 2))


def main() -> None:
    tracemalloc.start()
    index = build_index(50_000)
    current, peak = tracemalloc.get_traced_memory()
    top = tracemalloc.take_snapshot().statistics("lineno")[0]
    tracemalloc.stop()
    print(f"index holds {len(index)} entries")
    print(f"tracemalloc saw allocations: {current > 0 and peak >= current}")
    print(f"biggest allocation site: {top.traceback[0].filename.rsplit('/', 1)[-1]}")

    profiler = cProfile.Profile()
    profiler.enable()
    found = many_searches()
    profiler.disable()
    names = pstats.Stats(profiler).get_stats_profile().func_profiles
    print(f"matches found: {found}")
    print(f"profiler recorded slow_search: {'slow_search' in names}")

    # Dump every thread's stack on a fatal signal (SIGSEGV, SIGABRT...); stderr by default.
    faulthandler.enable(file=sys.stderr)
    print(f"faulthandler enabled: {faulthandler.is_enabled()}")
    faulthandler.dump_traceback_later(timeout=60, exit=False)  # watchdog for hangs
    faulthandler.cancel_dump_traceback_later()


if __name__ == "__main__":
    main()
