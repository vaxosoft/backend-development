use m11_ex1_password_hasher::{MAX_LEN, PasswordError, PasswordService, PolicyError, check_policy};

// Cheap parameters keep the tests fast; production uses PasswordService::new().
fn cheap() -> PasswordService {
    PasswordService::with_cost(8, 1)
}

#[test]
fn hash_is_a_phc_string_without_the_plaintext() {
    let hash = cheap().hash("correct horse battery").unwrap();
    assert!(hash.starts_with("$argon2id$"), "got {hash}");
    assert!(!hash.contains("horse"));
}

#[test]
fn every_hash_gets_a_fresh_salt() {
    let svc = cheap();
    let a = svc.hash("same password").unwrap();
    let b = svc.hash("same password").unwrap();
    assert_ne!(a, b);
    assert!(svc.verify("same password", &a).unwrap());
    assert!(svc.verify("same password", &b).unwrap());
}

#[test]
fn wrong_password_is_false_not_an_error() {
    let svc = cheap();
    let hash = svc.hash("s3cret-passw0rd").unwrap();
    assert_eq!(svc.verify("s3cret-passw0rd", &hash), Ok(true));
    assert_eq!(svc.verify("S3cret-passw0rd", &hash), Ok(false));
    assert_eq!(svc.verify("", &hash), Ok(false));
}

#[test]
fn malformed_stored_hash_is_an_error() {
    assert_eq!(
        cheap().verify("whatever", "plaintext-in-the-db"),
        Err(PasswordError::Malformed)
    );
}

#[test]
fn stored_parameters_win_over_the_current_configuration() {
    // A hash made with the old cost still verifies after the service is retuned.
    let old = PasswordService::with_cost(8, 1)
        .hash("rotate-me-please")
        .unwrap();
    let new = PasswordService::with_cost(16, 2);
    assert!(new.verify("rotate-me-please", &old).unwrap());
}

#[test]
fn needs_rehash_detects_weaker_hashes() {
    let weak = PasswordService::with_cost(8, 1)
        .hash("rotate-me-please")
        .unwrap();
    let strong = PasswordService::with_cost(16, 2);
    let fresh = strong.hash("rotate-me-please").unwrap();
    assert!(strong.needs_rehash(&weak));
    assert!(!strong.needs_rehash(&fresh));
    assert!(!PasswordService::with_cost(8, 1).needs_rehash(&fresh));
}

#[test]
fn policy_counts_characters_not_bytes() {
    assert_eq!(check_policy("short"), Err(PolicyError::TooShort));
    assert_eq!(check_policy("пароль"), Err(PolicyError::TooShort)); // 6 chars, 12 bytes
    assert_eq!(check_policy("парольпароль"), Ok(())); // 12 chars
    assert_eq!(check_policy("12345678"), Ok(()));
    assert_eq!(check_policy(&"x".repeat(MAX_LEN)), Ok(()));
    assert_eq!(
        check_policy(&"x".repeat(MAX_LEN + 1)),
        Err(PolicyError::TooLong)
    );
}
