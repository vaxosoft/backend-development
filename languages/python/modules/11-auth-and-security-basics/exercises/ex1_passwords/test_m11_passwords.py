import base64
import hashlib
import hmac

import pytest
from m11_passwords import hash_password, needs_rehash, verify_password

FAST = 2**4  # a cheap cost factor keeps the tests fast; production uses the default


def test_stored_format_carries_the_parameters() -> None:
    stored = hash_password("pw", n=FAST)

    scheme, n, r, p, salt, digest = stored.split("$")
    assert (scheme, n, r, p) == ("scrypt", str(FAST), "8", "1")
    assert len(base64.b64decode(salt)) == 16
    assert len(base64.b64decode(digest)) == 32


def test_matches_a_reference_scrypt_computation_for_a_fixed_salt() -> None:
    salt = b"0123456789abcdef"

    stored = hash_password("pw", salt=salt, n=FAST)

    expected = hashlib.scrypt(b"pw", salt=salt, n=FAST, r=8, p=1, dklen=32)
    assert stored.split("$")[5] == base64.b64encode(expected).decode()


def test_same_password_gets_a_different_salt_each_time() -> None:
    assert hash_password("pw", n=FAST) != hash_password("pw", n=FAST)


def test_verify_accepts_the_right_password_only() -> None:
    stored = hash_password("correct horse", n=FAST)

    assert verify_password("correct horse", stored) is True
    assert verify_password("correct horsf", stored) is False
    assert verify_password("", stored) is False


def test_verify_uses_the_cost_stored_in_the_hash_not_the_default() -> None:
    old = hash_password("pw", n=2**3)
    new = hash_password("pw", n=2**5)

    assert verify_password("pw", old) is True
    assert verify_password("pw", new) is True


@pytest.mark.parametrize(
    "stored",
    [
        "",
        "plaintext",
        "scrypt$16$8$1$onlyfive",
        "bcrypt$16$8$1$c2FsdA==$aGFzaA==",
        "scrypt$many$8$1$c2FsdA==$aGFzaA==",
        "scrypt$16$8$1$!!!$aGFzaA==",
        "scrypt$16$8$1$c2FsdA==$",
    ],
)
def test_malformed_hashes_are_rejected_without_raising(stored: str) -> None:
    assert verify_password("pw", stored) is False


def test_comparison_is_constant_time(monkeypatch: pytest.MonkeyPatch) -> None:
    calls: list[tuple[bytes, bytes]] = []
    real = hmac.compare_digest

    def spy(a: bytes, b: bytes) -> bool:
        calls.append((a, b))
        return real(a, b)

    monkeypatch.setattr(hmac, "compare_digest", spy)

    assert verify_password("pw", hash_password("pw", n=FAST)) is True
    assert len(calls) == 1


def test_needs_rehash_flags_other_costs_and_garbage() -> None:
    stored = hash_password("pw", n=FAST)

    assert needs_rehash(stored, n=FAST) is False
    assert needs_rehash(stored, n=FAST * 2) is True
    assert needs_rehash("not a hash") is True
