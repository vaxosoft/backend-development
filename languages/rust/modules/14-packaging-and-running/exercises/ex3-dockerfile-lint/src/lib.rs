//! Exercise 3: a small Dockerfile linter for Rust services. Docker is not needed: the Dockerfile is text.

#[derive(Debug, Clone, Copy, PartialEq, Eq, PartialOrd, Ord)]
pub enum Rule {
    /// Fewer than two `FROM` lines: the compiler would ship in the final image.
    SingleStage,
    /// A base image without a tag, or tagged `latest` (stage names, `scratch`, `@sha256:` digests and
    /// images whose name contains `$` are fine).
    UnpinnedBaseImage,
    /// The final stage has no `USER`, or its last `USER` is `root` or `0` (optionally `:group`).
    RunsAsRoot,
    /// The final stage has no `HEALTHCHECK` (or only `HEALTHCHECK NONE`).
    MissingHealthcheck,
    /// A `RUN` invokes `cargo build`, `cargo install`, `cargo test` or `cargo run` without `--locked`.
    CargoWithoutLocked,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct Finding {
    pub rule: Rule,
    /// 1-based line where the offending instruction starts.
    pub line: usize,
}

/// Lints a Dockerfile and returns findings sorted by line, then by rule.
///
/// Parsing notes: instruction keywords are case-insensitive; blank lines and `#` comments are ignored;
/// a trailing `\` continues the instruction on the next line (a comment line inside a continuation
/// is skipped); `FROM [--platform=...] image[:tag][@digest] [AS name]`.
///
/// Where a finding is reported: `SingleStage` at the first `FROM` (line 1 if there is none),
/// `UnpinnedBaseImage` at that `FROM`, `RunsAsRoot` at the last `USER` or, when there is none, at the
/// final `FROM`, `MissingHealthcheck` at the final `FROM`, `CargoWithoutLocked` at the `RUN`.
pub fn lint(_dockerfile: &str) -> Vec<Finding> {
    todo!("join continuation lines into instructions, track stages, apply the five rules")
}
