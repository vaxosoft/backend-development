//! Unit tests and table-driven tests against the public API.

use m12_testing::{Status, ValidationError, slugify, validate_title};

#[test]
fn slugify_table() {
    let cases = [
        ("Write the docs", "write-the-docs"),
        ("  Leading and trailing  ", "leading-and-trailing"),
        ("Already-slugged", "already-slugged"),
        ("Many   spaces!!!", "many-spaces"),
        ("Ünïcode only keeps ascii", "n-code-only-keeps-ascii"),
        ("", ""),
        ("!!!", ""),
    ];
    for (input, expected) in cases {
        // the message names the failing row, which is what a table needs
        assert_eq!(slugify(input), expected, "input: {input:?}");
    }
}

#[test]
fn validate_title_table() {
    let long = "x".repeat(201);
    let cases: Vec<(&str, Result<String, ValidationError>)> = vec![
        ("  Ship it  ", Ok("Ship it".to_string())),
        ("", Err(ValidationError::TitleEmpty)),
        ("   ", Err(ValidationError::TitleEmpty)),
        (&long, Err(ValidationError::TitleTooLong)),
    ];
    for (input, expected) in cases {
        assert_eq!(
            validate_title(input),
            expected,
            "input: {:?}",
            input.chars().take(10).collect::<String>()
        );
    }
}

#[test]
fn status_round_trips_through_text() {
    for status in [Status::Todo, Status::InProgress, Status::Done] {
        assert_eq!(status.to_string().parse::<Status>(), Ok(status));
    }
}

// A test may return Result so `?` replaces unwrap.
#[test]
fn unknown_status_is_an_error() -> Result<(), String> {
    let err = "blocked"
        .parse::<Status>()
        .expect_err("blocked is not a status");
    assert!(err.contains("blocked"));
    Ok(())
}

#[test]
#[should_panic(expected = "TitleEmpty")]
fn unwrap_on_invalid_input_panics_with_the_reason() {
    validate_title("  ").unwrap();
}

#[test]
#[ignore = "slow: run with `cargo test -- --ignored`"]
fn slugify_handles_a_megabyte_title() {
    let slug = slugify(&"word ".repeat(200_000));
    assert!(slug.starts_with("word-word"));
}
