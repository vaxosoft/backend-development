//! Other formats with the same derive: CSV (flat rows) and TOML (config files).

use chrono::NaiveDate;
use serde::{Deserialize, Serialize};

#[derive(Debug, Serialize, Deserialize)]
struct Row {
    id: u32,
    title: String,
    due_date: Option<NaiveDate>,
    done: bool,
}

#[derive(Debug, Serialize, Deserialize)]
struct Config {
    name: String,
    server: Server,
    #[serde(default)]
    features: Vec<String>,
}

#[derive(Debug, Serialize, Deserialize)]
struct Server {
    host: String,
    port: u16,
}

fn main() -> Result<(), Box<dyn std::error::Error>> {
    // CSV out: the header comes from the field names; commas and quotes are escaped.
    let rows = vec![
        Row {
            id: 1,
            title: "Write docs, then review".into(),
            due_date: NaiveDate::from_ymd_opt(2030, 1, 31),
            done: false,
        },
        Row {
            id: 2,
            title: "Say \"hi\"".into(),
            due_date: None,
            done: true,
        },
    ];
    let mut writer = csv::Writer::from_writer(Vec::new());
    for row in &rows {
        writer.serialize(row)?;
    }
    let text = String::from_utf8(writer.into_inner()?)?;
    print!("{text}");

    // CSV in: records stream one at a time; an empty cell becomes None.
    let mut reader = csv::Reader::from_reader(text.as_bytes());
    for record in reader.deserialize::<Row>() {
        println!("{:?}", record?);
    }

    // CSV errors know which line failed.
    let bad = "id,title,due_date,done\n1,ok,,false\nx,broken,,true\n";
    for record in csv::Reader::from_reader(bad.as_bytes()).deserialize::<Row>() {
        if let Err(e) = record {
            println!("error: {e}");
        }
    }

    // TOML: the usual format for config files (module 09 builds on this).
    let config: Config = toml::from_str(
        r#"
        name = "tracker"

        [server]
        host = "127.0.0.1"
        port = 8080
        "#,
    )?;
    println!("{config:?}");
    print!("{}", toml::to_string(&config)?);
    println!(
        "{}",
        toml::from_str::<Config>("name = 1").unwrap_err().message()
    );
    Ok(())
}
