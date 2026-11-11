//! Argon2id password hashing: random salt, PHC string, verify, and spawn_blocking.

use argon2::Argon2;
use argon2::password_hash::phc::PasswordHash;
use argon2::password_hash::{PasswordHasher, PasswordVerifier};

fn hash_password(password: &str) -> Result<String, argon2::password_hash::Error> {
    // hash_password picks a fresh random salt on every call
    Ok(Argon2::default()
        .hash_password(password.as_bytes())?
        .to_string())
}

fn verify_password(password: &str, stored: &str) -> bool {
    let Ok(parsed) = PasswordHash::new(stored) else {
        return false;
    };
    // The cost parameters come from the stored hash, not from this Argon2 value
    Argon2::default()
        .verify_password(password.as_bytes(), &parsed)
        .is_ok()
}

#[tokio::main]
async fn main() {
    // Argon2 is deliberately slow and memory hungry: keep it off the async worker threads.
    let stored = tokio::task::spawn_blocking(|| hash_password("correct horse battery staple"))
        .await
        .expect("hashing task panicked")
        .expect("hashing failed");

    let prefix: Vec<&str> = stored.split('$').take(4).collect();
    println!("phc prefix: {}", prefix.join("$"));
    println!("contains plaintext: {}", stored.contains("horse"));

    let again = hash_password("correct horse battery staple").expect("hashing failed");
    println!("same password, same hash: {}", stored == again);

    let ok = verify_password("correct horse battery staple", &stored);
    let wrong = verify_password("Tr0ub4dor&3", &stored);
    let garbage = verify_password("anything", "not-a-phc-string");
    println!("right password: {ok}");
    println!("wrong password: {wrong}");
    println!("malformed hash: {garbage}");
}
