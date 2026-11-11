//! Secrets stay wrapped in `SecretString`, and the application is wired by hand in one place
//! (the composition root) from the validated config.
use std::collections::HashMap;
use std::net::SocketAddr;
use std::sync::{Arc, Mutex};
use std::time::{SystemTime, UNIX_EPOCH};

use secrecy::{ExposeSecret, SecretString};

#[derive(Debug)]
struct Config {
    addr: SocketAddr,
    admin_token: SecretString,
}

impl Config {
    fn from_lookup(lookup: impl Fn(&str) -> Option<String>) -> Result<Self, String> {
        let addr = lookup("TT_ADDR")
            .unwrap_or_else(|| "127.0.0.1:8080".into())
            .parse()
            .map_err(|e| format!("TT_ADDR: {e}"))?;
        let token = lookup("TT_ADMIN_TOKEN").ok_or("TT_ADMIN_TOKEN is required")?;
        if token.len() < 8 {
            // Say what is wrong, never the value itself.
            return Err(format!(
                "TT_ADMIN_TOKEN must have at least 8 characters (got {})",
                token.len()
            ));
        }
        Ok(Self {
            addr,
            admin_token: SecretString::from(token),
        })
    }
}

trait Clock: Send + Sync {
    fn now_unix(&self) -> u64;
}

struct SystemClock;

impl Clock for SystemClock {
    fn now_unix(&self) -> u64 {
        SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .map_or(0, |d| d.as_secs())
    }
}

struct FixedClock(u64);

impl Clock for FixedClock {
    fn now_unix(&self) -> u64 {
        self.0
    }
}

trait TaskStore: Send + Sync {
    fn add(&self, title: &str, created_at: u64) -> u64;
    fn len(&self) -> usize;
}

#[derive(Default)]
struct MemoryStore(Mutex<Vec<(String, u64)>>);

impl TaskStore for MemoryStore {
    fn add(&self, title: &str, created_at: u64) -> u64 {
        let mut tasks = self.0.lock().expect("lock poisoned");
        tasks.push((title.to_owned(), created_at));
        tasks.len() as u64
    }

    fn len(&self) -> usize {
        self.0.lock().expect("lock poisoned").len()
    }
}

/// Shared application state: cheap to clone (it is all `Arc`s), which is what axum's `State` wants.
#[derive(Clone)]
struct App {
    store: Arc<dyn TaskStore>,
    clock: Arc<dyn Clock>,
    admin_token: Arc<SecretString>,
}

impl App {
    fn create_task(&self, title: &str) -> (u64, u64) {
        let created_at = self.clock.now_unix();
        (self.store.add(title, created_at), created_at)
    }

    fn is_admin(&self, presented: &str) -> bool {
        // The only place the secret is exposed. (Module 11 shows constant-time comparison.)
        presented == self.admin_token.expose_secret()
    }
}

/// The composition root: the one function that knows which concrete types are used.
fn build_app(config: &Config, clock: Arc<dyn Clock>) -> App {
    App {
        store: Arc::new(MemoryStore::default()),
        clock,
        admin_token: Arc::new(config.admin_token.clone()),
    }
}

fn main() {
    let env = |pairs: &'static [(&'static str, &'static str)]| {
        let map: HashMap<_, _> = pairs.iter().copied().collect();
        move |key: &str| map.get(key).map(|v| v.to_string())
    };

    println!("missing:  {}", Config::from_lookup(env(&[])).unwrap_err());
    println!(
        "too short: {}",
        Config::from_lookup(env(&[("TT_ADMIN_TOKEN", "abc")])).unwrap_err()
    );

    let config = Config::from_lookup(env(&[("TT_ADMIN_TOKEN", "correct-horse-battery")]))
        .expect("valid config");
    println!("debug:    {config:?}");
    println!("listens on {}", config.addr);

    // Tests and demos inject a FixedClock; production wiring passes `Arc::new(SystemClock)`.
    let app = build_app(&config, Arc::new(FixedClock(1_893_456_000)));
    println!("created:  {:?}", app.create_task("Write docs"));
    println!("created:  {:?}", app.create_task("Ship it"));
    println!("stored:   {}", app.store.len());
    println!(
        "admin?    right={} wrong={}",
        app.is_admin("correct-horse-battery"),
        app.is_admin("guess")
    );
    println!(
        "system clock is sane: {}",
        SystemClock.now_unix() > 1_700_000_000
    );
}
