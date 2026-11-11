use std::time::Duration;

use m07_ex2_retry_backoff::{RetryError, RetryPolicy, get_text};
use reqwest::Client;
use wiremock::matchers::{method, path};
use wiremock::{Mock, MockServer, ResponseTemplate};

fn policy(max_retries: u32) -> RetryPolicy {
    RetryPolicy {
        max_retries,
        base: Duration::from_millis(1),
        cap: Duration::from_millis(4),
    }
}

#[test]
fn delay_doubles_and_is_capped() {
    let p = RetryPolicy {
        max_retries: 5,
        base: Duration::from_millis(100),
        cap: Duration::from_millis(500),
    };
    let schedule: Vec<u128> = (0..5).map(|n| p.delay(n).as_millis()).collect();
    assert_eq!(schedule, [100, 200, 400, 500, 500]);
}

#[test]
fn delay_does_not_overflow_for_large_retry_counts() {
    let p = RetryPolicy {
        max_retries: 100,
        base: Duration::from_secs(1),
        cap: Duration::from_secs(30),
    };
    assert_eq!(p.delay(200), Duration::from_secs(30));
    assert_eq!(p.delay(u32::MAX), Duration::from_secs(30));
}

#[tokio::test]
async fn retries_5xx_until_it_succeeds() {
    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path("/flaky"))
        .respond_with(ResponseTemplate::new(503))
        .up_to_n_times(2)
        .expect(2)
        .mount(&server)
        .await;
    Mock::given(method("GET"))
        .and(path("/flaky"))
        .respond_with(ResponseTemplate::new(200).set_body_string("finally"))
        .expect(1)
        .mount(&server)
        .await;

    let body = get_text(
        &Client::new(),
        &format!("{}/flaky", server.uri()),
        &policy(3),
    )
    .await
    .unwrap();
    assert_eq!(body, "finally");
}

#[tokio::test]
async fn never_retries_a_4xx() {
    let server = MockServer::start().await;
    Mock::given(path("/missing"))
        .respond_with(ResponseTemplate::new(404))
        .expect(1)
        .mount(&server)
        .await;

    let error = get_text(
        &Client::new(),
        &format!("{}/missing", server.uri()),
        &policy(3),
    )
    .await
    .unwrap_err();
    assert_eq!(error, RetryError::Permanent(404));
}

#[tokio::test]
async fn too_many_requests_is_retryable() {
    let server = MockServer::start().await;
    Mock::given(path("/limited"))
        .respond_with(ResponseTemplate::new(429))
        .up_to_n_times(1)
        .expect(1)
        .mount(&server)
        .await;
    Mock::given(path("/limited"))
        .respond_with(ResponseTemplate::new(200).set_body_string("ok"))
        .expect(1)
        .mount(&server)
        .await;

    assert_eq!(
        get_text(
            &Client::new(),
            &format!("{}/limited", server.uri()),
            &policy(2)
        )
        .await
        .unwrap(),
        "ok"
    );
}

#[tokio::test]
async fn gives_up_after_max_retries_plus_one_attempts() {
    let server = MockServer::start().await;
    Mock::given(path("/down"))
        .respond_with(ResponseTemplate::new(500))
        .expect(3)
        .mount(&server)
        .await;

    let error = get_text(
        &Client::new(),
        &format!("{}/down", server.uri()),
        &policy(2),
    )
    .await
    .unwrap_err();
    assert_eq!(
        error,
        RetryError::Exhausted {
            attempts: 3,
            last: "status 500".into()
        }
    );
}

#[tokio::test]
async fn a_timeout_counts_as_a_retryable_failure() {
    let server = MockServer::start().await;
    Mock::given(path("/slow"))
        .respond_with(ResponseTemplate::new(200).set_delay(Duration::from_millis(300)))
        .expect(2)
        .mount(&server)
        .await;

    let client = Client::builder()
        .timeout(Duration::from_millis(20))
        .build()
        .unwrap();
    let error = get_text(&client, &format!("{}/slow", server.uri()), &policy(1))
        .await
        .unwrap_err();
    assert!(
        matches!(error, RetryError::Exhausted { attempts: 2, .. }),
        "got {error:?}"
    );
}
