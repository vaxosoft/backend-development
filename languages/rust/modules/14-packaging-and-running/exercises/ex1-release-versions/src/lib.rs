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
    pub fn parse(_text: &str) -> Result<Self, VersionError> {
        todo!(
            "strip an optional v, split off the pre-release at the first '-', parse three numbers"
        )
    }

    /// Bumping resets the lower parts and drops any pre-release. It moves forward from the numbers,
    /// so `1.4.2-rc.1` bumped by `Patch` is `1.4.3` and `1.4.2` bumped by `Minor` is `1.5.0`.
    pub fn bump(&self, _part: Part) -> Version {
        todo!()
    }

    /// Image tags for this release. Stable: `["1.4.2", "1.4", "1"]`, plus `"latest"` when `is_latest`.
    /// Pre-release: only the exact version, e.g. `["1.5.0-rc.1"]`, never a floating tag.
    pub fn image_tags(&self, _is_latest: bool) -> Vec<String> {
        todo!()
    }
}

impl fmt::Display for Version {
    fn fmt(&self, _f: &mut fmt::Formatter<'_>) -> fmt::Result {
        todo!("MAJOR.MINOR.PATCH with -PRE when present")
    }
}
