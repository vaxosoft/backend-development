use std::collections::HashMap;

use m09_ex2_layered_settings::{Layer, load};

fn lookup(pairs: &[(&str, &str)]) -> impl Fn(&str) -> Option<String> {
    let map: HashMap<String, String> = pairs
        .iter()
        .map(|(k, v)| (k.to_string(), v.to_string()))
        .collect();
    move |key| map.get(key).cloned()
}

fn addr(text: &str) -> std::net::SocketAddr {
    text.parse().unwrap()
}

#[test]
fn defaults_apply_when_no_layer_sets_anything() {
    let settings = load(None, lookup(&[]), Layer::default()).unwrap();
    assert_eq!(settings.addr, addr("127.0.0.1:8080"));
    assert_eq!(settings.timeout_ms, 5_000);
    assert_eq!(settings.database_url, "sqlite::memory:");
    assert_eq!(settings.workers, 4);
}

#[test]
fn from_toml_reads_the_known_keys() {
    let layer = Layer::from_toml("addr = \"0.0.0.0:9000\"\nworkers = 8\n").unwrap();
    assert_eq!(
        layer,
        Layer {
            addr: Some(addr("0.0.0.0:9000")),
            workers: Some(8),
            ..Layer::default()
        }
    );
}

#[test]
fn from_toml_rejects_typos_and_wrong_types() {
    let typo = Layer::from_toml("adr = \"0.0.0.0:9000\"").unwrap_err();
    assert!(
        typo.contains("adr"),
        "message should name the unknown key: {typo}"
    );
    assert!(Layer::from_toml("workers = \"many\"").is_err());
}

#[test]
fn from_lookup_leaves_unset_keys_empty() {
    let layer = Layer::from_lookup(lookup(&[("TT_TIMEOUT_MS", " 250 ")])).unwrap();
    assert_eq!(
        layer,
        Layer {
            timeout_ms: Some(250),
            ..Layer::default()
        }
    );
}

#[test]
fn from_lookup_names_the_bad_key() {
    let error = Layer::from_lookup(lookup(&[("TT_WORKERS", "lots")])).unwrap_err();
    assert!(error.contains("TT_WORKERS"), "{error}");
}

#[test]
fn merge_prefers_the_higher_layer_field_by_field() {
    let low = Layer {
        addr: Some(addr("1.1.1.1:1")),
        timeout_ms: Some(100),
        database_url: Some("sqlite:low".into()),
        workers: None,
    };
    let high = Layer {
        addr: None,
        timeout_ms: Some(200),
        database_url: None,
        workers: Some(2),
    };
    let merged = low.clone().merge(high);
    assert_eq!(
        merged,
        Layer {
            addr: Some(addr("1.1.1.1:1")),
            timeout_ms: Some(200),
            database_url: Some("sqlite:low".into()),
            workers: Some(2)
        }
    );
    assert_eq!(
        low.clone().merge(Layer::default()),
        low,
        "an empty layer changes nothing"
    );
}

#[test]
fn precedence_is_file_then_env_then_cli() {
    let file = "addr = \"10.0.0.1:1000\"\ntimeout_ms = 1000\nworkers = 2\ndatabase_url = \"sqlite:file.db\"\n";
    let env = lookup(&[
        ("TT_ADDR", "10.0.0.2:2000"),
        ("TT_TIMEOUT_MS", "2000"),
        ("TT_WORKERS", "3"),
    ]);
    let cli = Layer {
        addr: Some(addr("10.0.0.3:3000")),
        ..Layer::default()
    };

    let settings = load(Some(file), env, cli).unwrap();
    assert_eq!(
        settings.addr,
        addr("10.0.0.3:3000"),
        "cli beats env and file"
    );
    assert_eq!(settings.timeout_ms, 2000, "env beats file");
    assert_eq!(settings.workers, 3, "env beats file");
    assert_eq!(
        settings.database_url, "sqlite:file.db",
        "only the file set it"
    );
}

#[test]
fn validation_reports_every_problem_naming_the_field() {
    let cli = Layer {
        timeout_ms: Some(0),
        database_url: Some("mysql://db".into()),
        workers: Some(100),
        ..Layer::default()
    };
    let errors = load(None, lookup(&[]), cli).unwrap_err();
    assert_eq!(errors.len(), 3, "{errors:?}");
    assert!(errors[0].starts_with("timeout_ms"), "{errors:?}");
    assert!(errors[1].starts_with("database_url"), "{errors:?}");
    assert!(errors[2].starts_with("workers"), "{errors:?}");
}

#[test]
fn a_value_is_validated_after_merging_not_before() {
    // The file value is invalid, but the environment overrides it: that is a valid configuration.
    let settings = load(
        Some("workers = 500"),
        lookup(&[("TT_WORKERS", "5")]),
        Layer::default(),
    )
    .unwrap();
    assert_eq!(settings.workers, 5);
}

#[test]
fn broken_layers_are_reported_not_ignored() {
    let errors = load(
        Some("nonsense = true"),
        lookup(&[("TT_ADDR", "nope")]),
        Layer::default(),
    )
    .unwrap_err();
    assert_eq!(errors.len(), 2, "{errors:?}");
    assert!(errors.iter().any(|e| e.contains("nonsense")));
    assert!(errors.iter().any(|e| e.contains("TT_ADDR")));
}
