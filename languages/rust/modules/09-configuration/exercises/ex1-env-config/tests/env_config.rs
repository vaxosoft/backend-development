use std::collections::HashMap;
use std::time::Duration;

use m09_ex1_env_config::{AppConfig, Level, parse_bool, parse_duration};

fn lookup(pairs: &[(&str, &str)]) -> impl Fn(&str) -> Option<String> {
    let map: HashMap<String, String> = pairs
        .iter()
        .map(|(k, v)| (k.to_string(), v.to_string()))
        .collect();
    move |key| map.get(key).cloned()
}

#[test]
fn durations_with_units() {
    assert_eq!(parse_duration("500ms"), Ok(Duration::from_millis(500)));
    assert_eq!(parse_duration("30s"), Ok(Duration::from_secs(30)));
    assert_eq!(parse_duration(" 2m "), Ok(Duration::from_secs(120)));
    assert_eq!(parse_duration("0s"), Ok(Duration::ZERO));
}

#[test]
fn invalid_durations_are_rejected() {
    for input in ["", "30", "s", "-5s", "1.5s", "10h", "5 s", "ms500"] {
        assert!(
            parse_duration(input).is_err(),
            "{input:?} should be rejected"
        );
    }
}

#[test]
fn booleans() {
    for yes in ["true", "TRUE", "1", "yes", "On"] {
        assert_eq!(parse_bool(yes), Ok(true), "{yes}");
    }
    for no in ["false", "False", "0", "no", "OFF", " no "] {
        assert_eq!(parse_bool(no), Ok(false), "{no}");
    }
    for bad in ["", "maybe", "2", "tru"] {
        assert!(parse_bool(bad).is_err(), "{bad:?}");
    }
}

#[test]
fn only_the_database_url_is_required_and_everything_else_has_a_default() {
    let config =
        AppConfig::from_lookup(lookup(&[("TT_DATABASE_URL", "sqlite://tasks.db")])).unwrap();
    assert_eq!(config.addr, "127.0.0.1:8080".parse().unwrap());
    assert_eq!(config.database_url, "sqlite://tasks.db");
    assert_eq!(config.log_level, Level::Info);
    assert_eq!(config.request_timeout, Duration::from_secs(30));
    assert!(!config.signup_enabled);
}

#[test]
fn every_value_can_be_overridden() {
    let config = AppConfig::from_lookup(lookup(&[
        ("TT_ADDR", "0.0.0.0:9000"),
        ("TT_DATABASE_URL", "postgres://db/tasks"),
        ("TT_LOG_LEVEL", "DEBUG"),
        ("TT_REQUEST_TIMEOUT", "750ms"),
        ("TT_SIGNUP", "yes"),
    ]))
    .unwrap();
    assert_eq!(config.addr, "0.0.0.0:9000".parse().unwrap());
    assert_eq!(config.database_url, "postgres://db/tasks");
    assert_eq!(config.log_level, Level::Debug);
    assert_eq!(config.request_timeout, Duration::from_millis(750));
    assert!(config.signup_enabled);
}

#[test]
fn values_are_trimmed_and_blank_means_unset() {
    let config = AppConfig::from_lookup(lookup(&[
        ("TT_DATABASE_URL", "  sqlite://x  "),
        ("TT_ADDR", "   "),
        ("TT_LOG_LEVEL", ""),
    ]))
    .unwrap();
    assert_eq!(config.database_url, "sqlite://x");
    assert_eq!(config.addr, "127.0.0.1:8080".parse().unwrap());
    assert_eq!(config.log_level, Level::Info);
}

#[test]
fn a_missing_required_key_is_named_in_the_error() {
    let errors = AppConfig::from_lookup(lookup(&[])).unwrap_err();
    assert_eq!(errors.len(), 1);
    assert_eq!(errors[0].key, "TT_DATABASE_URL");
    assert!(errors[0].to_string().starts_with("TT_DATABASE_URL: "));
}

#[test]
fn all_problems_are_reported_at_once_in_key_order() {
    let errors = AppConfig::from_lookup(lookup(&[
        ("TT_ADDR", "localhost"),
        ("TT_LOG_LEVEL", "loud"),
        ("TT_REQUEST_TIMEOUT", "soon"),
        ("TT_SIGNUP", "maybe"),
    ]))
    .unwrap_err();
    let keys: Vec<&str> = errors.iter().map(|e| e.key).collect();
    assert_eq!(
        keys,
        [
            "TT_ADDR",
            "TT_DATABASE_URL",
            "TT_LOG_LEVEL",
            "TT_REQUEST_TIMEOUT",
            "TT_SIGNUP"
        ]
    );
}
