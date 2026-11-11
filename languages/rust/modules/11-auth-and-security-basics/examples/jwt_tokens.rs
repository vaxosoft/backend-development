//! Issuing and validating HS256 JWTs, and the validation settings that matter.

use base64::Engine;
use base64::engine::general_purpose::URL_SAFE_NO_PAD;
use jsonwebtoken::errors::ErrorKind;
use jsonwebtoken::{Algorithm, DecodingKey, EncodingKey, Header, Validation, decode, encode};
use m11_auth_and_security_basics::install_jwt_crypto;
use serde::{Deserialize, Serialize};

#[derive(Debug, Serialize, Deserialize)]
struct Claims {
    sub: String,
    role: String,
    exp: u64,
}

fn validation() -> Validation {
    // Pin the algorithm; never accept whatever the token header claims.
    let mut v = Validation::new(Algorithm::HS256);
    v.leeway = 0;
    v.set_required_spec_claims(&["exp", "sub"]);
    v
}

fn describe(result: jsonwebtoken::errors::Result<jsonwebtoken::TokenData<Claims>>) -> String {
    match result {
        Ok(data) => format!("ok sub={} role={}", data.claims.sub, data.claims.role),
        Err(e) => match e.kind() {
            ErrorKind::ExpiredSignature => "rejected: expired".to_string(),
            ErrorKind::InvalidSignature => "rejected: bad signature".to_string(),
            ErrorKind::InvalidAlgorithm => "rejected: wrong algorithm".to_string(),
            ErrorKind::MissingRequiredClaim(c) => format!("rejected: missing claim {c}"),
            other => format!("rejected: {other:?}"),
        },
    }
}

fn main() {
    install_jwt_crypto();
    let secret = b"a-long-random-secret-from-the-environment";
    let key = EncodingKey::from_secret(secret);
    let dec = DecodingKey::from_secret(secret);

    // Fixed timestamps keep the output reproducible: year 2100 and year 2001.
    let claims = |exp| Claims {
        sub: "7".into(),
        role: "member".into(),
        exp,
    };
    let valid = claims(4_102_444_800);
    let expired = claims(1_000_000_000);

    let token = encode(&Header::new(Algorithm::HS256), &valid, &key).expect("encode");
    println!("three segments: {}", token.split('.').count() == 3);
    println!(
        "valid token:    {}",
        describe(decode(&token, &dec, &validation()))
    );

    let old = encode(&Header::new(Algorithm::HS256), &expired, &key).expect("encode");
    println!(
        "expired token:  {}",
        describe(decode(&old, &dec, &validation()))
    );

    let other = DecodingKey::from_secret(b"some-other-secret");
    println!(
        "wrong secret:   {}",
        describe(decode(&token, &other, &validation()))
    );

    let segments: Vec<&str> = token.split('.').collect();

    // A member edits the payload to become admin: the signature no longer matches.
    let forged_payload = URL_SAFE_NO_PAD.encode(r#"{"sub":"7","role":"admin","exp":4102444800}"#);
    let tampered = format!("{}.{forged_payload}.{}", segments[0], segments[2]);
    println!(
        "tampered:       {}",
        describe(decode(&tampered, &dec, &validation()))
    );

    // Header says HS512 but we only accept HS256.
    let hs512 = format!("eyJhbGciOiJIUzUxMiJ9.{}.{}", segments[1], segments[2]);
    println!(
        "header HS512:   {}",
        describe(decode(&hs512, &dec, &validation()))
    );

    // Claims are signed, not encrypted: anyone can read them.
    let bytes = base64_url_decode(segments[1]);
    println!(
        "payload (readable by anyone): {}",
        String::from_utf8_lossy(&bytes)
    );
}

fn base64_url_decode(s: &str) -> Vec<u8> {
    URL_SAFE_NO_PAD.decode(s).unwrap_or_default()
}
