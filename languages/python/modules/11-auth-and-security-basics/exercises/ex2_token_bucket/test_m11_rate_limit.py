import pytest
from m11_rate_limit import TokenBucketLimiter


class FakeClock:
    def __init__(self) -> None:
        self.now = 1000.0

    def __call__(self) -> float:
        return self.now

    def advance(self, seconds: float) -> None:
        self.now += seconds


@pytest.fixture
def clock() -> FakeClock:
    return FakeClock()


def test_burst_is_allowed_then_requests_are_denied(clock: FakeClock) -> None:
    limiter = TokenBucketLimiter(burst=3, refill_per_second=1, clock=clock)

    decisions = [limiter.allow("ana") for _ in range(4)]

    assert [d.allowed for d in decisions] == [True, True, True, False]
    assert [d.remaining for d in decisions] == [2, 1, 0, 0]


def test_retry_after_tells_when_the_next_token_arrives(clock: FakeClock) -> None:
    limiter = TokenBucketLimiter(burst=1, refill_per_second=2, clock=clock)
    assert limiter.allow("ana").allowed

    assert limiter.allow("ana").retry_after == pytest.approx(0.5)
    clock.advance(0.25)
    assert limiter.allow("ana").retry_after == pytest.approx(0.25)
    clock.advance(0.25)
    assert limiter.allow("ana").allowed


def test_tokens_refill_over_time_but_never_beyond_the_burst(clock: FakeClock) -> None:
    limiter = TokenBucketLimiter(burst=2, refill_per_second=1, clock=clock)
    limiter.allow("ana")
    limiter.allow("ana")

    clock.advance(3600)  # an idle hour must not bank 3600 tokens

    assert [limiter.allow("ana").allowed for _ in range(3)] == [True, True, False]


def test_keys_have_independent_buckets(clock: FakeClock) -> None:
    limiter = TokenBucketLimiter(burst=1, refill_per_second=1, clock=clock)

    assert limiter.allow("ana").allowed
    assert not limiter.allow("ana").allowed
    assert limiter.allow("bob").allowed


def test_a_denied_request_does_not_consume_tokens(clock: FakeClock) -> None:
    limiter = TokenBucketLimiter(burst=2, refill_per_second=1, clock=clock)

    assert not limiter.allow("ana", cost=3).allowed
    assert limiter.allow("ana", cost=2).allowed


def test_prune_forgets_only_full_buckets(clock: FakeClock) -> None:
    limiter = TokenBucketLimiter(burst=2, refill_per_second=1, clock=clock)
    limiter.allow("busy")
    limiter.allow("busy")
    limiter.allow("quiet")
    assert limiter.prune() == 0

    clock.advance(1)  # quiet is full again, busy has 1 of 2

    assert limiter.prune() == 1
    assert limiter.allow("busy").remaining == 0  # busy kept its state


@pytest.mark.parametrize(("burst", "rate"), [(0, 1.0), (-1, 1.0), (1, 0.0), (1, -0.5)])
def test_invalid_configuration_is_rejected(burst: int, rate: float) -> None:
    with pytest.raises(ValueError, match="burst"):
        TokenBucketLimiter(burst=burst, refill_per_second=rate)
