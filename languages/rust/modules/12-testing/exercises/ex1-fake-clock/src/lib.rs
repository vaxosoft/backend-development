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
    #[allow(dead_code)] // used by your implementation
    now: Arc<Mutex<DateTime<Utc>>>,
}

impl FakeClock {
    pub fn new(start: DateTime<Utc>) -> Self {
        Self {
            now: Arc::new(Mutex::new(start)),
        }
    }

    pub fn advance(&self, _by: Duration) {
        todo!("move the shared time forward")
    }

    pub fn set(&self, _to: DateTime<Utc>) {
        todo!("jump to an absolute time")
    }
}

impl Clock for FakeClock {
    fn now(&self) -> DateTime<Utc> {
        todo!("read the shared time")
    }
}

/// Sessions with a sliding expiry: every successful `validate` pushes the expiry out by `ttl`.
#[allow(dead_code)] // the fields are used by your implementation
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
    pub fn create(&mut self, _user: &str) -> String {
        todo!("store the user with expiry = now + ttl")
    }

    /// The user behind a live session. A session is dead once `now >= expiry`; dead sessions are removed.
    pub fn validate(&mut self, _session_id: &str) -> Option<String> {
        todo!("check expiry against the clock, extend it when valid")
    }

    /// Removes all dead sessions and returns how many were removed.
    pub fn purge_expired(&mut self) -> usize {
        todo!("drop every session whose expiry has passed")
    }

    pub fn len(&self) -> usize {
        self.expires.len()
    }

    pub fn is_empty(&self) -> bool {
        self.expires.is_empty()
    }
}
