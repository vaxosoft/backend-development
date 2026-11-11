//! Exercise 1: make time injectable. Build a fake clock, then a component that depends on it.

use std::collections::HashMap;
use std::sync::{Arc, Mutex};

use chrono::{DateTime, Duration, Utc};

pub trait Clock: Send + Sync {
    fn now(&self) -> DateTime<Utc>;
}

pub struct SystemClock;

impl Clock for SystemClock {
    fn now(&self) -> DateTime<Utc> {
        Utc::now()
    }
}

/// A clock that only moves when told to. Clones share the same time, so a test can keep
/// one handle and give another to the component under test.
#[derive(Clone)]
pub struct FakeClock {
    now: Arc<Mutex<DateTime<Utc>>>,
}

impl FakeClock {
    pub fn new(start: DateTime<Utc>) -> Self {
        Self {
            now: Arc::new(Mutex::new(start)),
        }
    }

    pub fn advance(&self, by: Duration) {
        *self.now.lock().expect("clock mutex poisoned") += by;
    }

    pub fn set(&self, to: DateTime<Utc>) {
        *self.now.lock().expect("clock mutex poisoned") = to;
    }
}

impl Clock for FakeClock {
    fn now(&self) -> DateTime<Utc> {
        *self.now.lock().expect("clock mutex poisoned")
    }
}

/// Sessions with a sliding expiry: every successful `validate` pushes the expiry out by `ttl`.
pub struct SessionStore<C: Clock> {
    clock: C,
    ttl: Duration,
    next_id: u64,
    expires: HashMap<String, (String, DateTime<Utc>)>,
}

impl<C: Clock> SessionStore<C> {
    pub fn new(clock: C, ttl: Duration) -> Self {
        Self {
            clock,
            ttl,
            next_id: 0,
            expires: HashMap::new(),
        }
    }

    /// Returns a new session id ("s1", "s2", ...) for `user`.
    pub fn create(&mut self, user: &str) -> String {
        self.next_id += 1;
        let id = format!("s{}", self.next_id);
        let expiry = self.clock.now() + self.ttl;
        self.expires.insert(id.clone(), (user.to_string(), expiry));
        id
    }

    /// The user behind a live session. A session is dead once `now >= expiry`; dead sessions are removed.
    pub fn validate(&mut self, session_id: &str) -> Option<String> {
        let now = self.clock.now();
        let (user, expiry) = self.expires.get_mut(session_id)?;
        if now >= *expiry {
            self.expires.remove(session_id);
            return None;
        }
        *expiry = now + self.ttl;
        Some(user.clone())
    }

    /// Removes all dead sessions and returns how many were removed.
    pub fn purge_expired(&mut self) -> usize {
        let now = self.clock.now();
        let before = self.expires.len();
        self.expires.retain(|_, (_, expiry)| now < *expiry);
        before - self.expires.len()
    }

    pub fn len(&self) -> usize {
        self.expires.len()
    }

    pub fn is_empty(&self) -> bool {
        self.expires.is_empty()
    }
}
