//! Exercise 1: a password service built on Argon2id.

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
pub fn check_policy(_password: &str) -> Result<(), PolicyError> {
    todo!("reject passwords shorter than MIN_LEN or longer than MAX_LEN characters")
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

    /// Returns a PHC string (`$argon2id$...`) with a fresh random salt.
    pub fn hash(&self, _password: &str) -> Result<String, PasswordError> {
        let _argon2 = Argon2::from(&self.params);
        todo!("hash with a random salt and return the PHC string")
    }

    /// `Ok(false)` for a wrong password, `Err(Malformed)` when `stored` is not a PHC string.
    pub fn verify(&self, _password: &str, _stored: &str) -> Result<bool, PasswordError> {
        todo!("parse the stored hash, then verify")
    }

    /// True when `stored` was made with weaker memory or time cost than this service uses.
    pub fn needs_rehash(&self, _stored: &str) -> bool {
        todo!("compare the parameters in the stored hash with self.params")
    }
}

impl Default for PasswordService {
    fn default() -> Self {
        Self::new()
    }
}
