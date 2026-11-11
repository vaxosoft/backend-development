//! One upstream item, fetched with a timeout and retries.

use serde_json::Value;

use crate::Config;

/// Why an upstream call failed. `Display` is the text clients see in `errors[].error`.
#[derive(Debug, thiserror::Error)]
pub enum FetchError {
    #[error("upstream status {0}")]
    Status(u16),
    #[error("upstream timeout")]
    Timeout,
    #[error("upstream unreachable")]
    Network,
    #[error("invalid upstream response")]
    BadBody,
}

impl FetchError {
    fn is_retryable(&self) -> bool {
        match self {
            Self::Status(code) => *code >= 500,
            Self::Timeout | Self::Network => true,
            Self::BadBody => false,
        }
    }
}

async fn fetch_once(
    client: &reqwest::Client,
    config: &Config,
    id: u64,
) -> Result<Value, FetchError> {
    let url = format!("{}/items/{id}", config.upstream_base);
    // The request timeout covers connecting, the headers and reading the body.
    let response = client
        .get(url)
        .timeout(config.call_timeout)
        .send()
        .await
        .map_err(|e| {
            if e.is_timeout() {
                FetchError::Timeout
            } else {
                FetchError::Network
            }
        })?;
    let status = response.status();
    if !status.is_success() {
        return Err(FetchError::Status(status.as_u16()));
    }
    response.json().await.map_err(|e| {
        if e.is_timeout() {
            FetchError::Timeout
        } else if e.is_decode() {
            FetchError::BadBody
        } else {
            FetchError::Network
        }
    })
}

/// GET `{upstream_base}/items/{id}` and return its JSON body.
///
/// Network errors, timeouts and 5xx are retried with exponential backoff; 4xx never are.
pub async fn fetch_with_retry(
    client: &reqwest::Client,
    config: &Config,
    id: u64,
) -> Result<Value, FetchError> {
    let mut retries = 0;
    loop {
        match fetch_once(client, config, id).await {
            Err(err) if err.is_retryable() && retries < config.max_retries => {
                tokio::time::sleep(config.backoff_base * 2u32.pow(retries)).await;
                retries += 1;
            }
            outcome => return outcome,
        }
    }
}
