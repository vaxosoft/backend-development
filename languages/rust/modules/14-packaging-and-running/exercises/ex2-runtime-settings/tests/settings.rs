use std::collections::HashMap;
use std::time::Duration;

use m14_ex2_runtime_settings::{Environment, LogLevel, Settings, SettingsError, resolve};

fn run(args: &[&str], env: &[(&str, &str)]) -> Result<Settings, SettingsError> {
    let vars: HashMap<String, String> = env
        .iter()
        .map(|(k, v)| (k.to_string(), v.to_string()))
        .collect();
    resolve(args, &|key| vars.get(key).cloned())
}

fn invalid(field: &'static str, value: &str, reason: &'static str) -> SettingsError {
    SettingsError::Invalid {
        field,
        value: value.to_string(),
        reason,
    }
}

#[test]
fn development_defaults() {
    let s = run(&[], &[]).unwrap();
    assert_eq!(s.addr, "127.0.0.1:8080".parse().unwrap());
    assert_eq!(s.log_level, LogLevel::Debug);
    assert_eq!(s.workers, 4);
    assert_eq!(s.shutdown_timeout, Duration::from_secs(10));
    assert_eq!(s.environment, Environment::Development);
}

#[test]
fn production_defaults_to_info_logging() {
    let s = run(&[], &[("APP_ENV", "production")]).unwrap();
    assert_eq!(s.environment, Environment::Production);
    assert_eq!(s.log_level, LogLevel::Info);
}

#[test]
fn environment_overrides_defaults() {
    let s = run(
        &[],
        &[
            ("ADDR", "0.0.0.0:9000"),
            ("WORKERS", "8"),
            ("LOG_LEVEL", "warn"),
            ("SHUTDOWN_TIMEOUT_SECS", "30"),
        ],
    )
    .unwrap();
    assert_eq!(s.addr, "0.0.0.0:9000".parse().unwrap());
    assert_eq!(
        (s.workers, s.log_level, s.shutdown_timeout),
        (8, LogLevel::Warn, Duration::from_secs(30))
    );
}

#[test]
fn flags_override_environment_in_both_syntaxes() {
    let env = [("ADDR", "0.0.0.0:9000"), ("WORKERS", "8")];
    let s = run(&["--addr", "127.0.0.1:7000", "--workers=2"], &env).unwrap();
    assert_eq!(s.addr, "127.0.0.1:7000".parse().unwrap());
    assert_eq!(s.workers, 2);
}

#[test]
fn last_repeated_flag_wins() {
    assert_eq!(
        run(&["--workers", "2", "--workers", "6"], &[])
            .unwrap()
            .workers,
        6
    );
}

#[test]
fn unknown_and_incomplete_flags_are_reported() {
    assert_eq!(
        run(&["--bogus", "1"], &[]),
        Err(SettingsError::UnknownFlag("--bogus".into()))
    );
    assert_eq!(
        run(&["workers", "2"], &[]),
        Err(SettingsError::UnknownFlag("workers".into()))
    );
    assert_eq!(
        run(&["--workers"], &[]),
        Err(SettingsError::MissingValue("--workers".into()))
    );
    assert_eq!(
        run(&["--addr", "127.0.0.1:1", "--log-level"], &[]),
        Err(SettingsError::MissingValue("--log-level".into()))
    );
}

#[test]
fn invalid_values_name_the_field_and_the_offending_text() {
    assert_eq!(
        run(&["--addr", ":80"], &[]),
        Err(invalid("addr", ":80", "expected host:port"))
    );
    assert_eq!(
        run(&["--workers", "0"], &[]),
        Err(invalid("workers", "0", "out of range"))
    );
    assert_eq!(
        run(&["--workers", "257"], &[]),
        Err(invalid("workers", "257", "out of range"))
    );
    assert_eq!(
        run(&["--workers", "many"], &[]),
        Err(invalid("workers", "many", "not a whole number"))
    );
    assert_eq!(
        run(&["--workers", "-1"], &[]),
        Err(invalid("workers", "-1", "not a whole number"))
    );
    assert_eq!(
        run(&["--shutdown-timeout-secs", "301"], &[]),
        Err(invalid("shutdown-timeout-secs", "301", "out of range"))
    );
    assert_eq!(
        run(&["--log-level", "trace"], &[]),
        Err(invalid(
            "log-level",
            "trace",
            "expected debug, info, warn or error"
        ))
    );
    assert_eq!(
        run(&["--env", "staging"], &[]),
        Err(invalid(
            "env",
            "staging",
            "expected development or production"
        ))
    );
}

#[test]
fn an_invalid_environment_variable_is_an_error_too() {
    assert_eq!(
        run(&[], &[("WORKERS", "lots")]),
        Err(invalid("workers", "lots", "not a whole number"))
    );
    // ...but a valid flag takes precedence, so the bad variable is never consulted
    assert_eq!(
        run(&["--workers", "3"], &[("WORKERS", "lots")])
            .unwrap()
            .workers,
        3
    );
}

#[test]
fn production_refuses_debug_logging_from_any_source() {
    let rule = Err(SettingsError::Production("log level debug is not allowed"));
    assert_eq!(
        run(&["--env", "production", "--log-level", "debug"], &[]),
        rule
    );
    assert_eq!(
        run(&[], &[("APP_ENV", "production"), ("LOG_LEVEL", "debug")]),
        rule
    );
    // a flag can fix what the environment got wrong
    let s = run(
        &["--log-level", "warn"],
        &[("APP_ENV", "production"), ("LOG_LEVEL", "debug")],
    )
    .unwrap();
    assert_eq!(s.log_level, LogLevel::Warn);
    // and development may use debug freely
    assert!(run(&["--log-level", "debug"], &[]).is_ok());
}
