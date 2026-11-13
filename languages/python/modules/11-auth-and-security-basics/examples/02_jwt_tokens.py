"""JWT: signing, expiry, and the attacks `algorithms=[...]` and `require` defend against."""

import base64
import json
from datetime import timedelta

import jwt
from m11_security import decode_token, issue_token

SECRET = "demo-secret-demo-secret-demo-secret-32+"
EXPIRED = 946_684_800  # 2000-01-01T00:00:00Z


def attempt(label: str, token: str, secret: str = SECRET) -> None:
    try:
        claims = decode_token(token, secret)
        print(f"{label}: accepted sub={claims['sub']} role={claims['role']}")
    except jwt.InvalidTokenError as err:
        print(f"{label}: rejected ({type(err).__name__})")


def main() -> None:
    good = issue_token(7, "admin", SECRET)
    attempt("valid token", good)

    attempt("wrong secret", good, secret="another-secret-another-secret-32+++")

    user = issue_token(8, "user", SECRET)
    header, _, signature = user.split(".")
    claims = {"sub": "8", "role": "admin", "exp": 4_102_444_800}
    forged = base64.urlsafe_b64encode(json.dumps(claims).encode()).rstrip(b"=").decode()
    attempt("forged payload", f"{header}.{forged}.{signature}")

    expired = jwt.encode({"sub": "7", "role": "admin", "exp": EXPIRED}, SECRET, algorithm="HS256")
    attempt("expired", expired)

    no_exp = jwt.encode({"sub": "7", "role": "admin"}, SECRET, algorithm="HS256")
    attempt("no exp claim", no_exp)

    unsigned = jwt.encode({"sub": "7", "role": "admin", "exp": 4_102_444_800}, None, "none")
    attempt("alg=none", unsigned)

    short = issue_token(7, "user", SECRET, ttl=timedelta(minutes=5))
    peek = jwt.decode(short, options={"verify_signature": False})
    print(f"payload is only encoded, not encrypted: role={peek['role']}")


if __name__ == "__main__":
    main()
