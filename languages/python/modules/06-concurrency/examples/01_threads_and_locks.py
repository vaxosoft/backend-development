"""Threads: shared state needs a lock; a lock-ordering deadlock shown safely with timeouts."""

import threading
import time
from collections.abc import Callable
from concurrent.futures import ThreadPoolExecutor

N_THREADS = 8
N_INCREMENTS = 50_000


class Counter:
    def __init__(self) -> None:
        self.value = 0
        self._lock = threading.Lock()

    def unsafe_increment(self) -> None:
        self.value += 1  # read, add, write: another thread can slip in between

    def safe_increment(self) -> None:
        with self._lock:
            self.value += 1


def hammer(step: Callable[[], None]) -> None:
    for _ in range(N_INCREMENTS):
        step()


def run(safe: bool) -> int:
    counter = Counter()
    step = counter.safe_increment if safe else counter.unsafe_increment
    threads = [threading.Thread(target=hammer, args=(step,)) for _ in range(N_THREADS)]
    for t in threads:
        t.start()
    for t in threads:
        t.join()
    return counter.value


expected = N_THREADS * N_INCREMENTS
print("locked == expected:", run(safe=True) == expected)
# The unsafe total may or may not lose updates (it depends on the interpreter and timing),
# but it can never exceed the expected value.
print("unsafe <= expected:", run(safe=False) <= expected)


# A pool is the usual way to run blocking calls concurrently; map() keeps the input order.
def slow_square(n: int) -> int:
    time.sleep(0.01)
    return n * n


with ThreadPoolExecutor(max_workers=4) as pool:
    print(list(pool.map(slow_square, range(8))))

# Deadlock: two threads take the same two locks in opposite order. Acquire with a timeout
# so the demo ends; in real code, always take locks in one global order.
lock_a, lock_b = threading.Lock(), threading.Lock()
both_hold_one = threading.Barrier(2)
both_tried = threading.Barrier(2)
outcomes: list[str] = []


def take(first: threading.Lock, second: threading.Lock, name: str) -> None:
    with first:
        both_hold_one.wait()
        if second.acquire(timeout=0.05):
            second.release()
            outcomes.append(f"{name}: got both")
        else:
            outcomes.append(f"{name}: gave up (would deadlock)")
        both_tried.wait()  # keep holding `first` until the other thread has also given up


t1 = threading.Thread(target=take, args=(lock_a, lock_b, "t1"))
t2 = threading.Thread(target=take, args=(lock_b, lock_a, "t2"))
t1.start()
t2.start()
t1.join()
t2.join()
print(sorted(outcomes))
