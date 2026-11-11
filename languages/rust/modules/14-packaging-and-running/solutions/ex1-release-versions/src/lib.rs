//! Exercise 1: release versions and container image tags.
//!
//! A release pipeline turns a git tag like `v1.4.2` into image tags. Stable releases move the
//! "floating" tags (`1.4`, `1`, `latest`); pre-releases publish only their exact tag.

use std::fmt;

#[derive(Debug, PartialEq, Eq, thiserror::Error)]
pub enum VersionError {
    #[error(
        "version must look like MAJOR.MINOR.PATCH[-PRERELEASE] (optionally prefixed with v), got {0:?}"
    )]
    Invalid(String),
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Version {
    pub major: u64,
    pub minor: u64,
    pub patch: u64,
    /// e.g. `rc.1`; `None` for a stable release.
    pub pre: Option<String>,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum Part {
    Major,
    Minor,
    Patch,
}

impl Version {
    /// Accepts `1.4.2`, `v1.4.2` and `1.5.0-rc.1`. Rejects missing parts, non-numeric parts, leading
    /// zeros (`01`), an empty pre-release (`1.0.0-`), characters outside `[0-9A-Za-z.-]` in the
    /// pre-release, and build metadata (`+...`, which Docker tags cannot contain).
    pub fn parse(text: &str) -> Result<Self, VersionError> {
        let invalid = || VersionError::Invalid(text.to_string());
        let body = text.strip_prefix('v').unwrap_or(text);
        let (core, pre) = match body.split_once('-') {
            Some((core, pre)) => (core, Some(pre)),
            None => (body, None),
        };

        let numbers: Vec<&str> = core.split('.').collect();
        let [major, minor, patch] = numbers.as_slice() else {
            return Err(invalid());
        };
        let number = |s: &str| -> Result<u64, VersionError> {
            let plain = !s.is_empty() && s.bytes().all(|b| b.is_ascii_digit());
            if !plain || (s.len() > 1 && s.starts_with('0')) {
                return Err(invalid());
            }
            s.parse().map_err(|_| invalid())
        };

        if let Some(pre) = pre {
            let allowed = |c: char| c.is_ascii_alphanumeric() || c == '.' || c == '-';
            if pre.is_empty() || !pre.chars().all(allowed) {
                return Err(invalid());
            }
        }
        Ok(Self {
            major: number(major)?,
            minor: number(minor)?,
            patch: number(patch)?,
            pre: pre.map(str::to_string),
        })
    }

    /// Bumping resets the lower parts and drops any pre-release. It moves forward from the numbers,
    /// so `1.4.2-rc.1` bumped by `Patch` is `1.4.3` and `1.4.2` bumped by `Minor` is `1.5.0`.
    pub fn bump(&self, part: Part) -> Version {
        let (major, minor, patch) = (self.major, self.minor, self.patch);
        let (major, minor, patch) = match part {
            Part::Major => (major + 1, 0, 0),
            Part::Minor => (major, minor + 1, 0),
            Part::Patch => (major, minor, patch + 1),
        };
        Version {
            major,
            minor,
            patch,
            pre: None,
        }
    }

    /// Image tags for this release. Stable: `["1.4.2", "1.4", "1"]`, plus `"latest"` when `is_latest`.
    /// Pre-release: only the exact version, e.g. `["1.5.0-rc.1"]`, never a floating tag.
    pub fn image_tags(&self, is_latest: bool) -> Vec<String> {
        if self.pre.is_some() {
            return vec![self.to_string()];
        }
        let mut tags = vec![
            self.to_string(),
            format!("{}.{}", self.major, self.minor),
            self.major.to_string(),
        ];
        if is_latest {
            tags.push("latest".to_string());
        }
        tags
    }
}

impl fmt::Display for Version {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "{}.{}.{}", self.major, self.minor, self.patch)?;
        match &self.pre {
            Some(pre) => write!(f, "-{pre}"),
            None => Ok(()),
        }
    }
}
