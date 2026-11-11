//! API keys: random, shown once, stored only as a keyed hash, compared in constant time.

use std::collections::HashMap;

use base64::Engine;
use base64::engine::general_purpose::URL_SAFE_NO_PAD;
use hmac::{Hmac, KeyInit, Mac};
use rand::RngExt;
use sha2::Sha256;

type HmacSha256 = Hmac<Sha256>;

/// What the server keeps. The secret half of the key is never stored.
struct StoredKey {
    owner_id: u32,
    digest: Vec<u8>,
}

struct ApiKeys {
    pepper: Vec<u8>,
    by_id: HashMap<String, StoredKey>,
}

impl ApiKeys {
    fn new(pepper: &[u8]) -> Self {
        Self {
            pepper: pepper.to_vec(),
            by_id: HashMap::new(),
        }
    }

    fn mac(&self, secret: &str) -> HmacSha256 {
        let mut mac = HmacSha256::new_from_slice(&self.pepper).expect("HMAC accepts any key size");
        mac.update(secret.as_bytes());
        mac
    }

    /// Returns `tt_<id>_<secret>`. This is the only time the secret exists in full.
    fn issue(&mut self, owner_id: u32) -> String {
        let mut rng = rand::rng();
        let mut id = [0u8; 6];
        let mut secret = [0u8; 32];
        rng.fill(&mut id);
        rng.fill(&mut secret);
        let id = URL_SAFE_NO_PAD.encode(id);
        let secret = URL_SAFE_NO_PAD.encode(secret);
        let digest = self.mac(&secret).finalize().into_bytes().to_vec();
        self.by_id
            .insert(id.clone(), StoredKey { owner_id, digest });
        format!("tt_{id}_{secret}")
    }

    fn authenticate(&self, presented: &str) -> Option<u32> {
        let rest = presented.strip_prefix("tt_")?;
        let (id, secret) = rest.split_once('_')?;
        let stored = self.by_id.get(id)?;
        // verify_slice compares in constant time; `==` on the digests would not
        self.mac(secret).verify_slice(&stored.digest).ok()?;
        Some(stored.owner_id)
    }
}

fn main() {
    let mut keys = ApiKeys::new(b"server-side-pepper-from-the-environment");
    let key = keys.issue(42);

    println!("key length: {}", key.len());
    println!("key prefix: {}", &key[..3]);
    println!("authenticate(valid key):   {:?}", keys.authenticate(&key));

    let mut forged = key.clone();
    forged.pop();
    forged.push(if key.ends_with('A') { 'B' } else { 'A' });
    println!(
        "authenticate(altered key): {:?}",
        keys.authenticate(&forged)
    );
    println!(
        "authenticate(garbage):     {:?}",
        keys.authenticate("hello")
    );

    let other = keys.issue(43);
    println!("two keys differ: {}", key != other);
    println!("second key owner: {:?}", keys.authenticate(&other));
}
