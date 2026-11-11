//! Exercise 1: a password service built on Argon2id.

use argon2::password_hash::phc::PasswordHash;
use argon2::password_hash::{PasswordHasher, PasswordVerifier};
use argon2::{Argon2, Params};

pub const MIN_LEN: usize = 8;
pub const MAX_LEN: usize = 128;

#[derive(Debug, PartialEq, Eq, thiserror::Error)]
pub enum PolicyError {
    #[error("password must have at least {MIN_LEN} characters")]
    TooShort,
    #[error("password must have at most {MAX_LEN} characters")]
    TooLong,
}

#[derive(Debug, PartialEq, Eq, thiserror::Error)]
pub enum PasswordError {
    #[error("stored hash is not a valid PHC string")]
    Malformed,
    #[error("hashing failed")]
    Hashing,
}

/// Counts characters (not bytes) so non-ASCII passwords are treated fairly.
pub fn check_policy(password: &str) -> Result<(), PolicyError> {
    match password.chars().count() {
        n if n < MIN_LEN => Err(PolicyError::TooShort),
        n if n > MAX_LEN => Err(PolicyError::TooLong),
        _ => Ok(()),
    }
}

pub struct PasswordService {
    params: Params,
}

impl PasswordService {
    /// Production cost: the Argon2 defaults.
    pub fn new() -> Self {
        Self {
            params: Params::DEFAULT,
        }
    }

    /// Explicit cost, so tests can use a cheap setting. `m_cost` is in KiB.
    pub fn with_cost(m_cost: u32, t_cost: u32) -> Self {
        let params = Params::new(m_cost, t_cost, 1, None).expect("valid Argon2 parameters");
        Self { params }
    }

    fn argon2(&self) -> Argon2<'_> {
        Argon2::from(&self.params)
    }

    /// Returns a PHC string (`$argon2id$...`) with a fresh random salt.
    pub fn hash(&self, password: &str) -> Result<String, PasswordError> {
        self.argon2()
            .hash_password(password.as_bytes())
            .map(|h| h.to_string())
            .map_err(|_| PasswordError::Hashing)
    }

    /// `Ok(false)` for a wrong password, `Err(Malformed)` when `stored` is not a PHC string.
    pub fn verify(&self, password: &str, stored: &str) -> Result<bool, PasswordError> {
        let parsed = PasswordHash::new(stored).map_err(|_| PasswordError::Malformed)?;
        Ok(self
            .argon2()
            .verify_password(password.as_bytes(), &parsed)
            .is_ok())
    }

    /// True when `stored` was made with weaker memory or time cost than this service uses.
    pub fn needs_rehash(&self, stored: &str) -> bool {
        let Ok(parsed) = PasswordHash::new(stored) else {
            return true;
        };
        match Params::try_from(&parsed) {
            Ok(p) => p.m_cost() < self.params.m_cost() || p.t_cost() < self.params.t_cost(),
            Err(_) => true,
        }
    }
}

impl Default for PasswordService {
    fn default() -> Self {
        Self::new()
    }
}
