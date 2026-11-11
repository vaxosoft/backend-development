use m14_ex3_dockerfile_lint::{Finding, Rule, lint};

const GOOD: &str = r#"
# syntax=docker/dockerfile:1
FROM rust:1.99-slim-bookworm AS build
WORKDIR /app
COPY Cargo.toml Cargo.lock ./
RUN mkdir src && echo 'fn main() {}' > src/main.rs \
    && cargo build --release --locked \
    && rm -rf src
COPY src ./src
RUN touch src/main.rs && cargo build --release --locked

FROM gcr.io/distroless/cc-debian12:nonroot
COPY --from=build /app/target/release/tasks-service /usr/local/bin/tasks-service
EXPOSE 8080
USER nonroot:nonroot
HEALTHCHECK --interval=15s CMD ["/usr/local/bin/tasks-service", "--healthcheck"]
ENTRYPOINT ["/usr/local/bin/tasks-service"]
"#;

fn rules(text: &str) -> Vec<(Rule, usize)> {
    lint(text)
        .into_iter()
        .map(|Finding { rule, line }| (rule, line))
        .collect()
}

#[test]
fn a_well_formed_dockerfile_has_no_findings() {
    assert_eq!(rules(GOOD), []);
}

#[test]
fn a_naive_single_stage_dockerfile_breaks_most_rules() {
    let bad =
        "FROM rust:latest\nCOPY . .\nRUN cargo build --release\nCMD [\"./target/release/app\"]\n";
    assert_eq!(
        rules(bad),
        [
            (Rule::SingleStage, 1),
            (Rule::UnpinnedBaseImage, 1),
            (Rule::RunsAsRoot, 1),
            (Rule::MissingHealthcheck, 1),
            (Rule::CargoWithoutLocked, 3),
        ]
    );
}

#[test]
fn an_empty_file_is_a_single_stage_problem_only() {
    assert_eq!(rules(""), [(Rule::SingleStage, 1)]);
}

#[test]
fn base_images_must_be_pinned() {
    let cases = [
        ("rust", true),
        ("rust:latest", true),
        ("docker.io/library/rust", true),
        ("registry.local:5000/team/rust", true),
        ("rust:1.99", false),
        ("registry.local:5000/team/rust:1.99", false),
        ("debian@sha256:0123abcd", false),
        ("debian:12@sha256:0123abcd", false),
        ("scratch", false),
        ("${BASE_IMAGE}", false),
    ];
    for (image, unpinned) in cases {
        let text = format!("FROM {image} AS build\nFROM {image}\nUSER app\nHEALTHCHECK CMD true\n");
        let flagged = rules(&text)
            .iter()
            .any(|(r, _)| *r == Rule::UnpinnedBaseImage);
        assert_eq!(flagged, unpinned, "image {image}");
    }
}

#[test]
fn stage_names_and_platform_flags_are_understood() {
    let text = "FROM --platform=linux/amd64 rust:1.99 AS build\nFROM build AS final\nUSER app\nHEALTHCHECK CMD true\n";
    assert_eq!(rules(text), []);
}

#[test]
fn only_the_final_stage_decides_user_and_healthcheck() {
    let text = "FROM rust:1.99 AS build\nUSER root\nHEALTHCHECK CMD true\nFROM debian:12\nUSER 1000\nHEALTHCHECK CMD true\n";
    assert_eq!(rules(text), []);

    let text = "FROM rust:1.99 AS build\nUSER app\nHEALTHCHECK CMD true\nFROM debian:12\n";
    assert_eq!(
        rules(text),
        [(Rule::RunsAsRoot, 4), (Rule::MissingHealthcheck, 4)]
    );
}

#[test]
fn explicit_root_is_reported_at_the_user_line() {
    for user in ["root", "0", "root:root", "0:0"] {
        let text =
            format!("FROM rust:1.99 AS build\nFROM debian:12\nUSER {user}\nHEALTHCHECK CMD true\n");
        let expected = if user == "ROOT" {
            vec![]
        } else {
            vec![(Rule::RunsAsRoot, 3)]
        };
        assert_eq!(rules(&text), expected, "USER {user}");
    }
}

#[test]
fn the_last_user_instruction_wins() {
    let text = "FROM rust:1.99 AS build\nFROM debian:12\nUSER app\nRUN true\nUSER root\nHEALTHCHECK CMD true\n";
    assert_eq!(rules(text), [(Rule::RunsAsRoot, 5)]);
}

#[test]
fn healthcheck_none_does_not_count() {
    let text = "FROM rust:1.99 AS build\nFROM debian:12\nUSER app\nHEALTHCHECK NONE\n";
    assert_eq!(rules(text), [(Rule::MissingHealthcheck, 2)]);
}

#[test]
fn cargo_commands_need_locked_even_in_chains_and_continuations() {
    let text = "\
FROM rust:1.99 AS build
RUN cargo fetch && cargo build --release
RUN cargo test \\
    --workspace
RUN cargo +nightly install tasks --locked
RUN cargo fmt --check && echo cargo build is only text here
RUN apt-get update; cargo run
FROM debian:12
USER app
HEALTHCHECK CMD true
";
    assert_eq!(
        rules(text),
        [
            (Rule::CargoWithoutLocked, 2),
            (Rule::CargoWithoutLocked, 3),
            (Rule::CargoWithoutLocked, 7)
        ]
    );
}

#[test]
fn keywords_are_case_insensitive_and_comments_are_ignored() {
    let text = "\
from rust:1.99 as build
# RUN cargo build --release
run cargo build --release --locked
From debian:12
user app
healthcheck cmd true
";
    assert_eq!(rules(text), []);
}

#[test]
fn findings_are_ordered_by_line_then_rule() {
    let text = "FROM rust\nRUN cargo build\nFROM debian\nRUN cargo test\n";
    let got = rules(text);
    let lines: Vec<usize> = got.iter().map(|(_, l)| *l).collect();
    let mut sorted = lines.clone();
    sorted.sort();
    assert_eq!(lines, sorted);
    assert_eq!(got[0], (Rule::UnpinnedBaseImage, 1));
}
