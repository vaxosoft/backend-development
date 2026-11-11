use std::collections::HashMap;

use p7_tested_service_docker_ci::config::{Config, ConfigError, Environment, LogLevel};

/// Builds a fake environment; the tests never touch the real one.
fn load(vars: &[(&str, &str)]) -> Result<Config, ConfigError> {
    let env: HashMap<String, String> = vars
        .iter()
        .map(|(k, v)| ((*k).to_owned(), (*v).to_owned()))
        .collect();
    Config::from_lookup(|name| env.get(name).cloned())
}

#[test]
fn empty_environment_gives_the_defaults() {
    let cfg = load(&[]).unwrap();
    assert_eq!(cfg.port, 8080);
    assert_eq!(cfg.log_level, LogLevel::Info);
    assert_eq!(cfg.env, Environment::Dev);
}

#[test]
fn valid_values_are_read() {
    let cfg = load(&[("PORT", "9090"), ("LOG_LEVEL", "debug"), ("ENV", "prod")]).unwrap();
    assert_eq!(cfg.port, 9090);
    assert_eq!(cfg.log_level, LogLevel::Debug);
    assert_eq!(cfg.env, Environment::Prod);
}

#[test]
fn non_numeric_port_is_rejected_with_a_message_naming_variable_and_value() {
    let err = load(&[("PORT", "eighty")]).unwrap_err();
    assert_eq!(err.var, "PORT");
    let message = err.to_string();
    assert!(
        message.contains("PORT") && message.contains("eighty"),
        "{message}"
    );
}

#[test]
fn port_outside_1_to_65535_is_rejected() {
    for bad in ["0", "65536", "-1"] {
        let err = load(&[("PORT", bad)]).expect_err(bad);
        assert_eq!(err.var, "PORT", "{bad}");
    }
}

#[test]
fn unknown_log_level_is_rejected() {
    let err = load(&[("LOG_LEVEL", "verbose")]).unwrap_err();
    assert_eq!(err.var, "LOG_LEVEL");
    assert_eq!(err.value, "verbose");
}

#[test]
fn unknown_environment_name_is_rejected() {
    let err = load(&[("ENV", "qa")]).unwrap_err();
    assert_eq!(err.var, "ENV");
}
