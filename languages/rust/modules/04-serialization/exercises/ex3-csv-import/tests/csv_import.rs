use chrono::NaiveDate;
use m04_ex3_csv_import::{ImportError, Status, TaskRow, from_csv, to_csv};

fn sample() -> Vec<TaskRow> {
    vec![
        TaskRow {
            id: 1,
            title: "Write docs, then review".into(),
            status: Status::InProgress,
            due_date: NaiveDate::from_ymd_opt(2030, 1, 31),
            tags: vec!["docs".into(), "p1".into()],
            owner_id: Some(7),
        },
        TaskRow {
            id: 2,
            title: "Say \"hi\"".into(),
            status: Status::Todo,
            due_date: None,
            tags: vec![],
            owner_id: None,
        },
    ]
}

const HEADER: &str = "id,title,status,due_date,tags,owner_id\n";

#[test]
fn export_writes_header_and_joins_tags() {
    let csv = to_csv(&sample()).unwrap();
    assert_eq!(
        csv,
        format!(
            "{HEADER}1,\"Write docs, then review\",in_progress,2030-01-31,docs;p1,7\n2,\"Say \"\"hi\"\"\",todo,,,\n"
        )
    );
}

#[test]
fn export_then_import_round_trips() {
    let rows = sample();
    assert_eq!(from_csv(&to_csv(&rows).unwrap()).unwrap(), rows);
}

#[test]
fn empty_cells_become_none_and_empty_tags() {
    let rows = from_csv(&format!("{HEADER}5,Plain,done,,,\n")).unwrap();
    assert_eq!(rows[0].due_date, None);
    assert_eq!(rows[0].owner_id, None);
    assert!(rows[0].tags.is_empty());
}

#[test]
fn bad_status_is_a_csv_error() {
    let err = from_csv(&format!("{HEADER}1,x,blocked,,,\n")).unwrap_err();
    assert!(matches!(err, ImportError::Csv(_)), "{err}");
}

#[test]
fn validation_reports_row_and_field() {
    let input = format!("{HEADER}1,Fine,todo,,a;b,\n2,   ,todo,,,\n");
    match from_csv(&input).unwrap_err() {
        ImportError::Invalid { row, field, .. } => assert_eq!((row, field), (2, "title")),
        other => panic!("unexpected error: {other}"),
    }
}

#[test]
fn tag_rules_are_enforced() {
    let upper = from_csv(&format!("{HEADER}1,T,todo,,Docs,\n")).unwrap_err();
    assert!(upper.to_string().contains("lowercase"), "{upper}");

    let dup = from_csv(&format!("{HEADER}1,T,todo,,a;a,\n")).unwrap_err();
    assert!(dup.to_string().contains("unique"), "{dup}");

    let many = (0..11)
        .map(|i| format!("t{i}"))
        .collect::<Vec<_>>()
        .join(";");
    let too_many = from_csv(&format!("{HEADER}1,T,todo,,{many},\n")).unwrap_err();
    assert!(too_many.to_string().contains("at most 10"), "{too_many}");

    let empty_tag = from_csv(&format!("{HEADER}1,T,todo,,a;;b,\n")).unwrap_err();
    assert!(empty_tag.to_string().contains("1-30"), "{empty_tag}");
}
