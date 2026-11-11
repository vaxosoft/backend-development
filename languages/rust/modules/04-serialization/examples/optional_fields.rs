//! Optional values, defaults, unknown fields and PATCH-style `Option<Option<T>>`.

use serde::{Deserialize, Deserializer, Serialize};

#[derive(Debug, Serialize, Deserialize)]
struct Settings {
    name: String,
    #[serde(default)]
    retries: u32,
    #[serde(default = "default_timeout")]
    timeout_secs: u32,
    #[serde(skip_serializing_if = "Option::is_none")]
    note: Option<String>,
}

fn default_timeout() -> u32 {
    30
}

#[derive(Debug, Deserialize)]
#[serde(deny_unknown_fields)]
struct Strict {
    name: String,
}

/// Plain `Option<Option<T>>` cannot tell `null` from "absent"; this can.
fn double_option<'de, D, T>(d: D) -> Result<Option<Option<T>>, D::Error>
where
    D: Deserializer<'de>,
    T: Deserialize<'de>,
{
    Option::<T>::deserialize(d).map(Some)
}

#[derive(Debug, Deserialize)]
struct NaivePatch {
    due: Option<Option<String>>,
}

#[derive(Debug, Deserialize)]
struct Patch {
    #[serde(default, deserialize_with = "double_option")]
    due: Option<Option<String>>,
}

fn main() -> Result<(), serde_json::Error> {
    // Missing Option field = None; `#[serde(default)]` fills other missing fields.
    let s: Settings = serde_json::from_str(r#"{"name": "api"}"#)?;
    println!("{s:?}");
    println!("{}", serde_json::to_string(&s)?);

    // `skip_serializing_if` omits the key; without it you would get "note":null.
    let s = Settings {
        note: Some("hi".into()),
        ..s
    };
    println!("{}", serde_json::to_string(&s)?);

    // null is accepted for Option, but not for a plain type.
    let s: Settings = serde_json::from_str(r#"{"name": "api", "note": null}"#)?;
    println!("{:?}", s.note);
    let err = serde_json::from_str::<Settings>(r#"{"name": null}"#).unwrap_err();
    println!("{err}");

    // Unknown fields are ignored by default, rejected on request.
    let strict: Strict = serde_json::from_str(r#"{"name": "a"}"#)?;
    println!("{}", strict.name);
    let err = serde_json::from_str::<Strict>(r#"{"name": "a", "extra": 1}"#).unwrap_err();
    println!("{err}");

    // PATCH: absent / null / value.
    for body in [r#"{}"#, r#"{"due": null}"#, r#"{"due": "2030-01-31"}"#] {
        let naive: NaivePatch = serde_json::from_str(body)?;
        let patch: Patch = serde_json::from_str(body)?;
        println!(
            "{body:<24} naive={:?}  double_option={:?}",
            naive.due, patch.due
        );
    }
    Ok(())
}
