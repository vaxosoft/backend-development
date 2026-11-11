use chrono::{TimeZone, Utc};
use m04_ex1_create_task_json::{Status, Task, parse_create_task, task_to_json};

#[test]
fn status_uses_snake_case_names() {
    let names: Vec<String> = [Status::Todo, Status::InProgress, Status::Done]
        .iter()
        .map(|s| serde_json::to_string(s).unwrap())
        .collect();
    assert_eq!(names, [r#""todo""#, r#""in_progress""#, r#""done""#]);
}

#[test]
fn unknown_status_is_rejected() {
    let err = serde_json::from_str::<Status>(r#""blocked""#).unwrap_err();
    assert!(
        err.to_string().contains("unknown variant `blocked`"),
        "{err}"
    );
}

#[test]
fn only_title_is_required() {
    let input = parse_create_task(r#"{"title":"Write docs"}"#).unwrap();
    assert_eq!(input.title, "Write docs");
    assert_eq!(input.status, Status::Todo);
    assert_eq!(input.due_date, None);
    assert!(input.tags.is_empty());
    assert_eq!(input.owner_id, None);
}

#[test]
fn full_body_is_decoded() {
    let body = r#"{"title":"Ship","status":"in_progress","due_date":"2030-01-31","tags":["p1"],"owner_id":7}"#;
    let input = parse_create_task(body).unwrap();
    assert_eq!(input.status, Status::InProgress);
    assert_eq!(input.due_date.unwrap().to_string(), "2030-01-31");
    assert_eq!(input.tags, ["p1"]);
    assert_eq!(input.owner_id, Some(7));
}

#[test]
fn missing_title_is_an_error() {
    let err = parse_create_task(r#"{"status":"done"}"#).unwrap_err();
    assert!(err.to_string().contains("missing field `title`"), "{err}");
}

#[test]
fn unknown_fields_are_rejected() {
    let err = parse_create_task(r#"{"title":"x","colour":"red"}"#).unwrap_err();
    assert!(err.to_string().contains("unknown field `colour`"), "{err}");
}

#[test]
fn task_serializes_to_the_documented_wire_format() {
    let input = parse_create_task(
        r#"{"title":"Write docs","status":"in_progress","due_date":"2030-01-31","tags":["docs","p1"]}"#,
    )
    .unwrap();
    let created_at = Utc.with_ymd_and_hms(2030, 1, 1, 9, 0, 0).unwrap();
    let task = Task::from_create(1, created_at, input);
    assert_eq!(
        task_to_json(&task),
        r#"{"id":1,"title":"Write docs","status":"in_progress","due_date":"2030-01-31","tags":["docs","p1"],"owner_id":null,"created_at":"2030-01-01T09:00:00Z"}"#
    );
}
