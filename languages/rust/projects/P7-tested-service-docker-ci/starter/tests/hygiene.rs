//! "Repo hygiene" tests: the packaging files are text, so their rules can be
//! checked without Docker or a CI runner.

use std::path::Path;

fn read(relative: &str) -> String {
    let path = Path::new(env!("CARGO_MANIFEST_DIR")).join(relative);
    std::fs::read_to_string(&path)
        .unwrap_or_else(|_| panic!("{relative} is missing: create it in the project directory"))
}

struct Instruction {
    keyword: String,
    args: String,
}

struct Stage {
    image: String,
    name: Option<String>,
    instructions: Vec<Instruction>,
}

impl Stage {
    fn find(&self, keyword: &str) -> impl Iterator<Item = &Instruction> {
        self.instructions
            .iter()
            .filter(move |i| i.keyword == keyword)
    }
}

/// Splits a Dockerfile into stages: comments dropped, `\` continuations joined.
fn stages(dockerfile: &str) -> Vec<Stage> {
    let mut logical = Vec::new();
    let mut current = String::new();
    for line in dockerfile.lines().map(str::trim) {
        if line.starts_with('#') || (line.is_empty() && current.is_empty()) {
            continue;
        }
        match line.strip_suffix('\\') {
            Some(part) => {
                current.push_str(part);
                current.push(' ');
            }
            None => {
                current.push_str(line);
                logical.push(std::mem::take(&mut current));
            }
        }
    }

    let mut stages: Vec<Stage> = Vec::new();
    for line in logical {
        let (keyword, args) = line.split_once(char::is_whitespace).unwrap_or((&line, ""));
        let keyword = keyword.to_ascii_uppercase();
        if keyword == "FROM" {
            let mut words = args.split_whitespace().filter(|w| !w.starts_with("--"));
            let image = words.next().unwrap_or_default().to_owned();
            let name = match (words.next(), words.next()) {
                (Some(as_kw), Some(name)) if as_kw.eq_ignore_ascii_case("as") => {
                    Some(name.to_owned())
                }
                _ => None,
            };
            stages.push(Stage {
                image,
                name,
                instructions: Vec::new(),
            });
        } else if let Some(stage) = stages.last_mut() {
            stage.instructions.push(Instruction {
                keyword,
                args: args.trim().to_owned(),
            });
        }
    }
    stages
}

fn dockerfile_stages() -> Vec<Stage> {
    let stages = stages(&read("Dockerfile"));
    assert!(!stages.is_empty(), "the Dockerfile has no FROM instruction");
    stages
}

#[test]
fn dockerfile_is_multi_stage_with_a_named_build_stage() {
    let stages = dockerfile_stages();
    assert!(stages.len() >= 2, "expected at least 2 FROM instructions");
    assert!(
        stages[0].name.is_some(),
        "name the first stage: FROM ... AS build"
    );
}

#[test]
fn build_stage_compiles_a_locked_release_binary() {
    let build = &dockerfile_stages()[0];
    let compiles = build.find("RUN").any(|run| {
        ["cargo build", "--release", "--locked"]
            .iter()
            .all(|part| run.args.contains(part))
    });
    assert!(
        compiles,
        "the build stage needs RUN cargo build --release --locked ..."
    );
}

#[test]
fn final_stage_is_a_minimal_runtime_image_that_copies_the_binary() {
    let stages = dockerfile_stages();
    let build_name = stages[0].name.clone().unwrap_or_default();
    let last = stages.last().unwrap();
    let base = last.image.rsplit('/').next().unwrap();
    assert!(
        !base.starts_with("rust"),
        "the toolchain image must not be the final image"
    );
    assert!(
        last.find("RUN").all(|run| !run.args.contains("cargo")),
        "the final stage must not build anything"
    );
    let copy_marker = format!("--from={build_name}");
    assert!(
        last.find("COPY")
            .any(|copy| copy.args.contains(&copy_marker)),
        "the final stage must COPY --from=<build stage> the binary"
    );
}

#[test]
fn base_images_are_pinned_to_a_version() {
    let stages = dockerfile_stages();
    for (i, stage) in stages.iter().enumerate() {
        let earlier_stage = stages[..i]
            .iter()
            .any(|s| s.name.as_deref() == Some(&stage.image));
        if stage.image == "scratch" || earlier_stage || stage.image.contains("@sha256:") {
            continue;
        }
        let tag = stage.image.rsplit_once(':').map(|(_, tag)| tag);
        assert!(
            tag.is_some_and(|t| !t.is_empty() && t != "latest"),
            "pin {} to a version tag or digest",
            stage.image
        );
    }
}

#[test]
fn final_stage_runs_as_a_non_root_user() {
    let stages = dockerfile_stages();
    let last = stages.last().unwrap();
    let user = last
        .find("USER")
        .last()
        .expect("the final stage needs a USER instruction");
    let name = user.args.split(':').next().unwrap();
    assert!(
        !matches!(name, "root" | "0"),
        "USER must not be root, got {}",
        user.args
    );
}

#[test]
fn final_stage_documents_the_port_and_a_health_probe() {
    let stages = dockerfile_stages();
    let last = stages.last().unwrap();
    assert!(
        last.find("EXPOSE")
            .any(|e| e.args.chars().next().is_some_and(|c| c.is_ascii_digit())),
        "EXPOSE the service port"
    );
    assert!(
        last.find("HEALTHCHECK")
            .any(|h| !h.args.eq_ignore_ascii_case("none")),
        "add a HEALTHCHECK"
    );
}

#[test]
fn dockerignore_keeps_build_output_and_git_data_out_of_the_context() {
    let ignored: Vec<String> = read(".dockerignore")
        .lines()
        .map(|l| {
            l.trim()
                .trim_start_matches("**/")
                .trim_end_matches('/')
                .to_owned()
        })
        .collect();
    for pattern in ["target", ".git"] {
        assert!(
            ignored.iter().any(|l| l == pattern),
            ".dockerignore must list {pattern}"
        );
    }
}

fn ci() -> String {
    read(".github/workflows/ci.yml")
}

fn ci_has_line(ci: &str, needle: &str) -> bool {
    ci.lines().any(|line| line.trim_start().starts_with(needle))
}

#[test]
fn ci_runs_on_push_and_pull_request() {
    let ci = ci();
    assert!(ci_has_line(&ci, "on:"), "missing the `on:` trigger block");
    assert!(ci_has_line(&ci, "push:"), "missing the push trigger");
    assert!(
        ci_has_line(&ci, "pull_request:"),
        "missing the pull_request trigger"
    );
}

#[test]
fn ci_checks_build_format_lint_and_tests() {
    let ci = ci();
    let run_lines: Vec<&str> = ci.lines().filter(|l| l.contains("cargo ")).collect();
    let has = |parts: &[&str]| {
        run_lines
            .iter()
            .any(|l| parts.iter().all(|p| l.contains(p)))
    };
    assert!(has(&["cargo build"]), "no build step");
    assert!(has(&["cargo fmt", "--check"]), "no format check");
    assert!(
        has(&["cargo clippy", "-D warnings"]),
        "no lint step that fails on warnings"
    );
    assert!(has(&["cargo test"]), "no test step");
}

#[test]
fn ci_pins_the_same_toolchain_as_the_dockerfile_builds_with() {
    let stages = dockerfile_stages();
    let image = &stages[0].image;
    let version = image
        .strip_prefix("rust:")
        .and_then(|tag| tag.split('-').next())
        .unwrap_or_else(|| panic!("the build stage must start from rust:<version>, got {image}"));

    let ci = ci();
    let pinned = ci
        .lines()
        .filter_map(|l| l.trim().strip_prefix("toolchain:"))
        .map(|v| v.trim().trim_matches(['"', '\'']).to_owned())
        .collect::<Vec<_>>();
    assert!(
        pinned.iter().any(|v| v == version),
        "ci.yml must set `toolchain: \"{version}\"` (found {pinned:?})"
    );
}
