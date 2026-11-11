//! Build metadata baked into the binary at compile time (see `build.rs`).

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct BuildInfo {
    pub name: &'static str,
    pub version: &'static str,
    pub profile: &'static str,
    pub target: &'static str,
    pub git_sha: Option<&'static str>,
}

/// Values captured when this crate was compiled.
pub const BUILD: BuildInfo = BuildInfo {
    name: env!("CARGO_PKG_NAME"),
    version: env!("CARGO_PKG_VERSION"),
    profile: env!("BUILD_PROFILE"),
    target: env!("BUILD_TARGET"),
    git_sha: option_env!("GIT_SHA"),
};
