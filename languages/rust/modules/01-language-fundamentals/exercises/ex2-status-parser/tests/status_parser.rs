use std::error::Error;

use m01_ex2_status_parser::{LineError, ParseError, Status, parse_all, parse_task};

#[test]
fn status_round_trips_through_text() {
    for status in [Status::Todo, Status::InProgress, Status::Done] {
        assert_eq!(status.to_string().parse::<Status>(), Ok(status));
    }
    assert_eq!(Status::InProgress.to_string(), "in_progress");
}

#[test]
fn unknown_status_is_an_error_value() {
    assert_eq!(
        "finished".parse::<Status>(),
        Err(ParseError::UnknownStatus("finished".into()))
    );
    assert!(
        "Todo".parse::<Status>().is_err(),
        "matching is case-sensitive"
    );
}

#[test]
fn next_follows_the_workflow() {
    assert_eq!(Status::Todo.next(), Some(Status::InProgress));
    assert_eq!(Status::InProgress.next(), Some(Status::Done));
    assert_eq!(Status::Done.next(), None);
}

#[test]
fn parse_task_trims_and_splits_tags() {
    let task = parse_task("  Write docs |in_progress| docs, p1 ,,").unwrap();
    assert_eq!(task.title, "Write docs");
    assert_eq!(task.status, Status::InProgress);
    assert_eq!(task.tags, vec!["docs", "p1"]);
}

#[test]
fn parse_task_allows_no_tags() {
    assert!(parse_task("Fix bug|todo|").unwrap().tags.is_empty());
}

#[test]
fn parse_task_reports_each_failure_kind() {
    assert_eq!(parse_task("only|two"), Err(ParseError::FieldCount(2)));
    assert_eq!(parse_task("a|todo|x|y"), Err(ParseError::FieldCount(4)));
    assert_eq!(parse_task("   |todo|"), Err(ParseError::EmptyTitle));
    assert_eq!(
        parse_task("t|nope|"),
        Err(ParseError::UnknownStatus("nope".into()))
    );
}

#[test]
fn parse_all_skips_blank_and_comment_lines() {
    let tasks = parse_all("# tasks\nA|todo|x\n\nB|done|\n").unwrap();
    let titles: Vec<_> = tasks.iter().map(|t| t.title.as_str()).collect();
    assert_eq!(titles, ["A", "B"]);
}

#[test]
fn parse_all_stops_at_first_error_with_line_number() {
    let err = parse_all("A|todo|\n\nB|oops|\nC|bad").unwrap_err();
    assert_eq!(
        err,
        LineError {
            line: 3,
            source: ParseError::UnknownStatus("oops".into())
        }
    );
    assert_eq!(err.to_string(), "line 3: unknown status 'oops'");
    assert!(
        err.source().is_some(),
        "the cause is preserved via source()"
    );
}
