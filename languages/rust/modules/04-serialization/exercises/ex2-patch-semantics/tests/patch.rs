use chrono::NaiveDate;
use m04_ex2_patch_semantics::{Status, Task, TaskPatch};

fn sample() -> Task {
    Task {
        id: 1,
        title: "Write docs".into(),
        status: Status::Todo,
        due_date: NaiveDate::from_ymd_opt(2030, 1, 31),
        tags: vec!["docs".into()],
    }
}

fn parse(body: &str) -> TaskPatch {
    serde_json::from_str(body).unwrap()
}

#[test]
fn absent_null_and_value_are_distinguished() {
    assert_eq!(parse("{}").due_date, None);
    assert_eq!(parse(r#"{"due_date":null}"#).due_date, Some(None));
    assert_eq!(
        parse(r#"{"due_date":"2030-02-01"}"#).due_date,
        Some(NaiveDate::from_ymd_opt(2030, 2, 1))
    );
}

#[test]
fn bad_date_is_still_an_error() {
    assert!(serde_json::from_str::<TaskPatch>(r#"{"due_date":"tomorrow"}"#).is_err());
}

#[test]
fn only_the_four_patchable_fields_are_accepted() {
    let err = serde_json::from_str::<TaskPatch>(r#"{"owner_id":3}"#).unwrap_err();
    assert!(
        err.to_string().contains("unknown field `owner_id`"),
        "{err}"
    );
}

#[test]
fn empty_patch_changes_nothing() {
    let mut task = sample();
    task.apply(parse("{}"));
    assert_eq!(task, sample());
}

#[test]
fn null_clears_the_due_date_and_keeps_the_rest() {
    let mut task = sample();
    task.apply(parse(r#"{"due_date":null}"#));
    assert_eq!(task.due_date, None);
    assert_eq!(task.title, "Write docs");
    assert_eq!(task.tags, ["docs"]);
}

#[test]
fn present_fields_replace_values() {
    let mut task = sample();
    task.apply(parse(
        r#"{"title":"Ship it","status":"done","due_date":"2031-05-06","tags":[]}"#,
    ));
    assert_eq!(task.title, "Ship it");
    assert_eq!(task.status, Status::Done);
    assert_eq!(task.due_date, NaiveDate::from_ymd_opt(2031, 5, 6));
    assert!(task.tags.is_empty());
}
