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

struct Instruction {
    line: usize,
    keyword: String,
    args: String,
}

fn parse(dockerfile: &str) -> Vec<Instruction> {
    let mut out = Vec::new();
    let mut current: Option<(usize, String)> = None;
    for (index, raw) in dockerfile.lines().enumerate() {
        let trimmed = raw.trim();
        if trimmed.is_empty() || trimmed.starts_with('#') {
            continue;
        }
        let (start, mut text) = current.take().unwrap_or((index + 1, String::new()));
        let continued = trimmed.ends_with('\\');
        let piece = trimmed.trim_end_matches('\\').trim();
        if !text.is_empty() {
            text.push(' ');
        }
        text.push_str(piece);
        if continued {
            current = Some((start, text));
        } else {
            out.push(split(start, &text));
        }
    }
    if let Some((start, text)) = current {
        out.push(split(start, &text));
    }
    out
}

fn split(line: usize, text: &str) -> Instruction {
    let (keyword, args) = text.split_once(char::is_whitespace).unwrap_or((text, ""));
    Instruction {
        line,
        keyword: keyword.to_ascii_uppercase(),
        args: args.trim().to_string(),
    }
}

/// `(image, stage name)` of a FROM instruction's arguments.
fn from_parts(args: &str) -> (&str, Option<&str>) {
    let mut words = args.split_whitespace().filter(|w| !w.starts_with("--"));
    let image = words.next().unwrap_or("");
    let name = match (words.next(), words.next()) {
        (Some(kw), Some(name)) if kw.eq_ignore_ascii_case("as") => Some(name),
        _ => None,
    };
    (image, name)
}

fn is_unpinned(image: &str, stages: &[String]) -> bool {
    if image == "scratch" || image.contains('$') || image.contains("@sha256:") {
        return false;
    }
    if stages.iter().any(|s| s.eq_ignore_ascii_case(image)) {
        return false;
    }
    // a ':' after the last '/' is a tag (a ':' before it would be a registry port)
    let name = image.rsplit('/').next().unwrap_or(image);
    match name.split_once(':') {
        None => true,
        Some((_, tag)) => tag == "latest",
    }
}

fn runs_cargo_without_locked(command: &str) -> bool {
    command
        .split(['&', ';', '|'])
        .map(str::trim)
        .filter(|segment| !segment.is_empty())
        .any(|segment| {
            let words: Vec<&str> = segment.split_whitespace().collect();
            words.first() == Some(&"cargo")
                && words
                    .iter()
                    .skip(1)
                    .find(|w| !w.starts_with('+') && !w.starts_with('-'))
                    .is_some_and(|sub| matches!(*sub, "build" | "install" | "test" | "run"))
                && !words.contains(&"--locked")
        })
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
pub fn lint(dockerfile: &str) -> Vec<Finding> {
    let instructions = parse(dockerfile);
    let mut findings = Vec::new();
    let mut push = |rule, line| findings.push(Finding { rule, line });

    let mut stages: Vec<String> = Vec::new();
    let froms: Vec<&Instruction> = instructions
        .iter()
        .filter(|i| i.keyword == "FROM")
        .collect();
    if froms.len() < 2 {
        push(Rule::SingleStage, froms.first().map_or(1, |f| f.line));
    }

    let mut final_from_line = 1;
    let mut last_user: Option<(&str, usize)> = None;
    let mut has_healthcheck = false;

    for ins in &instructions {
        match ins.keyword.as_str() {
            "FROM" => {
                let (image, name) = from_parts(&ins.args);
                if is_unpinned(image, &stages) {
                    push(Rule::UnpinnedBaseImage, ins.line);
                }
                stages.push(name.unwrap_or("").to_string());
                // per-stage state restarts with every FROM; only the last stage matters
                final_from_line = ins.line;
                last_user = None;
                has_healthcheck = false;
            }
            "USER" => last_user = Some((ins.args.as_str(), ins.line)),
            "HEALTHCHECK" => {
                has_healthcheck = !ins.args.trim().eq_ignore_ascii_case("none");
            }
            "RUN" if runs_cargo_without_locked(&ins.args) => {
                push(Rule::CargoWithoutLocked, ins.line)
            }
            _ => {}
        }
    }

    if !froms.is_empty() {
        match last_user {
            Some((user, line)) => {
                let name = user.split(':').next().unwrap_or("");
                if name == "root" || name == "0" {
                    push(Rule::RunsAsRoot, line);
                }
            }
            None => push(Rule::RunsAsRoot, final_from_line),
        }
        if !has_healthcheck {
            push(Rule::MissingHealthcheck, final_from_line);
        }
    }

    findings.sort_by_key(|f| (f.line, f.rule));
    findings
}
