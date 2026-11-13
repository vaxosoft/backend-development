"""Password hashing with scrypt: salted, slow, constant-time verification."""

import hmac

from m11_security import hash_password, verify_password


def main() -> None:
    first = hash_password("correct horse battery staple")
    second = hash_password("correct horse battery staple")

    scheme, n, r, p, salt, digest = first.split("$")
    print(f"scheme={scheme} n={n} r={r} p={p}")
    print(f"salt chars={len(salt)} hash chars={len(digest)}")
    print(f"same password, different hashes: {first != second}")

    print(f"right password verifies: {verify_password('correct horse battery staple', first)}")
    print(f"wrong password verifies: {verify_password('Tr0ub4dor&3', first)}")
    print(f"garbage hash verifies: {verify_password('x', 'not-a-hash')}")

    # Never compare secrets with ==; compare_digest takes the same time wherever they differ.
    print(f"compare_digest: {hmac.compare_digest(b'abc', b'abd')}")

    fixed = hash_password("pw", salt=b"0123456789abcdef")
    print(f"fixed salt is reproducible: {fixed == hash_password('pw', salt=b'0123456789abcdef')}")


if __name__ == "__main__":
    main()
