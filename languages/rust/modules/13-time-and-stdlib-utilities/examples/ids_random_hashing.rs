//! UUIDs, random numbers and hashing primitives. Random values are never printed, only their properties.

use hmac::{Hmac, KeyInit, Mac};
use rand::distr::Alphanumeric;
use rand::seq::{IndexedRandom, SliceRandom};
use rand::{Rng, RngExt, SeedableRng};
use sha2::{Digest, Sha256};
use uuid::{NoContext, Timestamp, Uuid};

fn main() {
    // UUID v4: 122 random bits. UUID v7: millisecond timestamp prefix, so ids sort by creation time.
    let v4 = Uuid::new_v4();
    println!(
        "v4 version={} text length={}",
        v4.get_version_num(),
        v4.to_string().len()
    );

    let early = Uuid::new_v7(Timestamp::from_unix(NoContext, 1_900_000_000, 0));
    let late = Uuid::new_v7(Timestamp::from_unix(NoContext, 1_900_000_001, 0));
    println!(
        "v7 version={} early<late: {}",
        early.get_version_num(),
        early < late
    );
    println!("v7 timestamp prefix: {}", &early.to_string()[..13]);
    println!(
        "parse round trip: {}",
        Uuid::parse_str(&v4.to_string()) == Ok(v4)
    );
    println!("bad uuid: {}", Uuid::parse_str("not-a-uuid").is_err());

    // rand::rng() is a fast CSPRNG seeded from the OS: fine for ids and tokens.
    let mut rng = rand::rng();
    let slug: String = (&mut rng)
        .sample_iter(Alphanumeric)
        .take(7)
        .map(char::from)
        .collect();
    println!(
        "slug length={} alphanumeric={}",
        slug.len(),
        slug.chars().all(|c| c.is_ascii_alphanumeric())
    );
    let roll = rng.random_range(1..=6);
    println!("die in 1..=6: {}", (1..=6).contains(&roll));

    // Seeded generators are reproducible: use them in tests and simulations, never for secrets.
    let mut a = rand::rngs::StdRng::seed_from_u64(7);
    let mut b = rand::rngs::StdRng::seed_from_u64(7);
    let mut deck = [1, 2, 3, 4, 5, 6, 7, 8];
    let mut deck2 = deck;
    deck.shuffle(&mut a);
    deck2.shuffle(&mut b);
    println!("same seed, same shuffle: {}", deck == deck2);
    println!(
        "choose from slice: {}",
        ["todo", "in_progress", "done"].choose(&mut a).is_some()
    );
    println!(
        "next_u32 bytes differ per call: {}",
        a.next_u32() != a.next_u32()
    );

    // Hashing: SHA-256 for integrity/ETags, HMAC-SHA-256 for authenticity (needs a secret key).
    let digest = Sha256::digest(b"hello tracker");
    println!("sha256: {}", hex::encode(digest));

    let mut mac = Hmac::<Sha256>::new_from_slice(b"shared-secret").unwrap();
    mac.update(b"task.created:42");
    let tag = mac.finalize().into_bytes();
    println!("hmac:   {}", hex::encode(tag));

    let mut check = Hmac::<Sha256>::new_from_slice(b"shared-secret").unwrap();
    check.update(b"task.created:42");
    println!("verify ok: {}", check.verify_slice(&tag).is_ok());
    let mut forged = Hmac::<Sha256>::new_from_slice(b"shared-secret").unwrap();
    forged.update(b"task.created:43");
    println!("verify forged: {}", forged.verify_slice(&tag).is_ok());
}
