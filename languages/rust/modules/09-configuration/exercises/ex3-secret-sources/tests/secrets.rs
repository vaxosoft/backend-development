use std::collections::HashMap;
use std::io;

use m09_ex3_secret_sources::{DbConfig, DbConfigError, SecretError, load_secret};
use secrecy::ExposeSecret;

fn env(pairs: &[(&str, &str)]) -> impl Fn(&str) -> Option<String> + use<> {
    let map: HashMap<String, String> = pairs
        .iter()
        .map(|(k, v)| (k.to_string(), v.to_string()))
        .collect();
    move |key| map.get(key).cloned()
}

#[test]
fn a_plain_variable_is_the_secret() {
    let secret = load_secret(env(&[("API_KEY", "abc 123\n")]), "API_KEY")
        .unwrap()
        .unwrap();
    assert_eq!(
        secret.expose_secret(),
        "abc 123\n",
        "env values are used unchanged"
    );
}

#[test]
fn nothing_set_means_none() {
    assert!(load_secret(env(&[]), "API_KEY").unwrap().is_none());
}

#[test]
fn a_file_secret_loses_exactly_one_trailing_newline() {
    let dir = tempfile::tempdir().unwrap();
    for (content, expected) in [
        ("hunter2\n", "hunter2"),
        ("hunter2\r\n", "hunter2"),
        ("hunter2", "hunter2"),
        ("hunter2\n\n", "hunter2\n"),
        ("  spaced  \n", "  spaced  "),
    ] {
        let path = dir.path().join("secret");
        std::fs::write(&path, content).unwrap();
        let path = path.to_string_lossy().into_owned();
        let secret = load_secret(env(&[("TOKEN_FILE", &path)]), "TOKEN")
            .unwrap()
            .unwrap();
        assert_eq!(secret.expose_secret(), expected, "file content {content:?}");
    }
}

#[test]
fn setting_both_forms_is_a_conflict() {
    let error = load_secret(
        env(&[("TOKEN", "a"), ("TOKEN_FILE", "/run/secrets/token")]),
        "TOKEN",
    )
    .unwrap_err();
    assert_eq!(
        error,
        SecretError::Conflict {
            name: "TOKEN".into()
        }
    );
}

#[test]
fn an_empty_secret_is_rejected_from_both_sources() {
    assert_eq!(
        load_secret(env(&[("TOKEN", "")]), "TOKEN").unwrap_err(),
        SecretError::Empty {
            name: "TOKEN".into()
        }
    );

    let dir = tempfile::tempdir().unwrap();
    let path = dir.path().join("empty");
    std::fs::write(&path, "\n").unwrap();
    let path = path.to_string_lossy().into_owned();
    assert_eq!(
        load_secret(env(&[("TOKEN_FILE", &path)]), "TOKEN").unwrap_err(),
        SecretError::Empty {
            name: "TOKEN".into()
        }
    );
}

#[test]
fn an_unreadable_file_reports_the_path_and_kind() {
    let dir = tempfile::tempdir().unwrap();
    let path = dir.path().join("missing").to_string_lossy().into_owned();
    let error = load_secret(env(&[("TOKEN_FILE", &path)]), "TOKEN").unwrap_err();
    assert_eq!(
        error,
        SecretError::Unreadable {
            name: "TOKEN".into(),
            path,
            kind: io::ErrorKind::NotFound
        }
    );
}

fn db_env(password: &str) -> impl Fn(&str) -> Option<String> + use<> {
    env(&[
        ("DB_NAME", "tasks"),
        ("DB_USER", "app"),
        ("DB_PASSWORD", password),
    ])
}

#[test]
fn db_config_builds_the_connection_string_and_a_redacted_twin() {
    let config = DbConfig::from_lookup(db_env("s3cret-pass")).unwrap();
    assert_eq!(config.host, "localhost");
    assert_eq!(
        config.connection_string().expose_secret(),
        "postgres://app:s3cret-pass@localhost/tasks"
    );
    assert_eq!(config.redacted_url(), "postgres://app:***@localhost/tasks");
}

#[test]
fn db_config_never_prints_the_password() {
    let config = DbConfig::from_lookup(db_env("s3cret-pass")).unwrap();
    for text in [
        format!("{config:?}"),
        format!("{:?}", config.connection_string()),
        config.redacted_url(),
    ] {
        assert!(!text.contains("s3cret-pass"), "leaked in {text}");
    }
}

#[test]
fn db_config_reads_the_password_from_a_file() {
    let dir = tempfile::tempdir().unwrap();
    let path = dir.path().join("db_password");
    std::fs::write(&path, "from-a-file-1\n").unwrap();
    let path = path.to_string_lossy().into_owned();
    let lookup = env(&[
        ("DB_NAME", "tasks"),
        ("DB_USER", "app"),
        ("DB_HOST", "db.internal"),
        ("DB_PASSWORD_FILE", &path),
    ]);

    let config = DbConfig::from_lookup(lookup).unwrap();
    assert_eq!(
        config.connection_string().expose_secret(),
        "postgres://app:from-a-file-1@db.internal/tasks"
    );
}

#[test]
fn db_config_validation_errors_do_not_leak_the_value() {
    let error = DbConfig::from_lookup(db_env("short")).unwrap_err();
    assert_eq!(error, DbConfigError::WeakPassword { min: 8 });
    assert!(!error.to_string().contains("short"));

    assert_eq!(
        DbConfig::from_lookup(env(&[("DB_USER", "app"), ("DB_PASSWORD", "long-enough")]))
            .unwrap_err(),
        DbConfigError::Missing("DB_NAME")
    );
    assert_eq!(
        DbConfig::from_lookup(env(&[("DB_NAME", "t"), ("DB_USER", "app")])).unwrap_err(),
        DbConfigError::Missing("DB_PASSWORD")
    );

    let conflict = DbConfig::from_lookup(env(&[
        ("DB_NAME", "t"),
        ("DB_USER", "u"),
        ("DB_PASSWORD", "long-enough"),
        ("DB_PASSWORD_FILE", "/x"),
    ]))
    .unwrap_err();
    assert!(matches!(
        conflict,
        DbConfigError::Secret(SecretError::Conflict { .. })
    ));
}
