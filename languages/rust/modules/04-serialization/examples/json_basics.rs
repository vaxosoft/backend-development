//! JSON basics: encode, decode, read errors, and the dynamic `Value` escape hatch.

use serde::{Deserialize, Serialize};
use serde_json::{Value, json};

#[derive(Debug, Serialize, Deserialize)]
struct Point {
    x: i32,
    y: i32,
}

#[derive(Debug, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
struct Profile {
    display_name: String,
    is_admin: bool,
    scores: Vec<u32>,
    nickname: Option<String>,
}

fn main() -> Result<(), serde_json::Error> {
    // Encode: the derive does all the work.
    let point = Point { x: 3, y: -4 };
    println!("{}", serde_json::to_string(&point)?);

    let profile = Profile {
        display_name: "Ada".into(),
        is_admin: true,
        scores: vec![10, 20],
        nickname: None,
    };
    println!("{}", serde_json::to_string(&profile)?);
    println!("{}", serde_json::to_string_pretty(&profile)?);

    // Decode: the target type decides what is accepted.
    let decoded: Point = serde_json::from_str(r#"{"x": 1, "y": 2, "ignored": true}"#)?;
    println!("{decoded:?}");

    // Errors carry the position and say what was expected.
    for bad in [
        r#"{"x": 1}"#,
        r#"{"x": "one", "y": 2}"#,
        r#"{"x": 1, "y": 2"#,
        r#"{"x": 1.5, "y": 2}"#,
    ] {
        let err = serde_json::from_str::<Point>(bad).unwrap_err();
        println!("{bad:<24} -> {err} ({:?})", err.classify());
    }

    // Untyped access when the shape is not known up front.
    let value: Value = serde_json::from_str(r#"{"user": {"name": "Ada", "tags": ["a", "b"]}}"#)?;
    println!("{}", value["user"]["name"]);
    println!("{:?}", value["user"]["tags"][1].as_str());
    println!("{}", value["user"]["missing"]);

    // `json!` builds a Value; handy for tests and ad-hoc responses.
    println!("{}", json!({"ok": true, "count": 2, "items": [1, 2]}));
    Ok(())
}
