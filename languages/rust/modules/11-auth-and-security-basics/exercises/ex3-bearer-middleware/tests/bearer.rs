use axum::Router;
use axum::body::Body;
use axum::http::{Request, StatusCode, header};
use base64::Engine;
use base64::engine::general_purpose::URL_SAFE_NO_PAD;
use http_body_util::BodyExt;
use m11_ex3_bearer_middleware::{Auth, Role, app};
use tower::ServiceExt;

const SECRET: &[u8] = b"exercise-secret-0123456789-abcdef";

async fn get(
    router: &Router,
    path: &str,
    authorization: Option<&str>,
) -> (StatusCode, Option<String>, String) {
    let mut req = Request::builder().uri(path);
    if let Some(value) = authorization {
        req = req.header(header::AUTHORIZATION, value);
    }
    let resp = router
        .clone()
        .oneshot(req.body(Body::empty()).unwrap())
        .await
        .unwrap();
    let status = resp.status();
    let challenge = resp
        .headers()
        .get(header::WWW_AUTHENTICATE)
        .map(|v| v.to_str().unwrap().to_string());
    let body = resp.into_body().collect().await.unwrap().to_bytes();
    (status, challenge, String::from_utf8(body.to_vec()).unwrap())
}

fn router() -> (Router, Auth) {
    let auth = Auth::new(SECRET);
    (app(auth.clone()), auth)
}

#[tokio::test]
async fn public_route_needs_no_token() {
    let (router, _) = router();
    let (status, _, body) = get(&router, "/healthz", None).await;
    assert_eq!((status, body.as_str()), (StatusCode::OK, "ok"));
}

#[tokio::test]
async fn valid_token_reaches_the_handler_with_claims() {
    let (router, auth) = router();
    let token = auth.issue("7", Role::Member, 60);
    let (status, _, body) = get(&router, "/me", Some(&format!("Bearer {token}"))).await;
    assert_eq!(status, StatusCode::OK);
    let json: serde_json::Value = serde_json::from_str(&body).unwrap();
    assert_eq!(json, serde_json::json!({"sub": "7", "role": "member"}));
}

#[tokio::test]
async fn scheme_is_case_insensitive() {
    let (router, auth) = router();
    let token = auth.issue("7", Role::Member, 60);
    let (status, ..) = get(&router, "/me", Some(&format!("bearer {token}"))).await;
    assert_eq!(status, StatusCode::OK);
}

#[tokio::test]
async fn missing_or_malformed_credentials_get_a_challenge() {
    let (router, auth) = router();
    let token = auth.issue("7", Role::Member, 60);
    let basic = format!("Basic {token}");
    for header_value in [
        None,
        Some("Bearer"),
        Some("Bearer "),
        Some("Token abc"),
        Some(basic.as_str()),
    ] {
        let (status, challenge, body) = get(&router, "/me", header_value).await;
        assert_eq!(status, StatusCode::UNAUTHORIZED, "header: {header_value:?}");
        assert_eq!(challenge.as_deref(), Some("Bearer"));
        assert!(body.contains(r#""code":"unauthorized""#), "body: {body}");
    }
}

#[tokio::test]
async fn expired_wrongly_signed_and_garbage_tokens_are_indistinguishable() {
    let (router, auth) = router();
    let expired = auth.issue("7", Role::Admin, -5);
    let foreign = Auth::new(b"some-other-secret-0123456789-abcd").issue("7", Role::Admin, 60);

    let mut bodies = Vec::new();
    for token in [expired.as_str(), foreign.as_str(), "not.a.jwt", "x"] {
        let (status, challenge, body) = get(&router, "/me", Some(&format!("Bearer {token}"))).await;
        assert_eq!(status, StatusCode::UNAUTHORIZED, "token: {token}");
        assert_eq!(challenge.as_deref(), Some("Bearer"));
        bodies.push(body);
    }
    assert!(
        bodies.windows(2).all(|w| w[0] == w[1]),
        "responses leak the failure reason: {bodies:?}"
    );
}

#[tokio::test]
async fn tokens_with_other_algorithms_are_rejected() {
    let (router, auth) = router();
    let valid = auth.issue("7", Role::Admin, 60);
    let payload = valid.split('.').nth(1).unwrap();
    let signature = valid.split('.').nth(2).unwrap();

    let header_hs512 = URL_SAFE_NO_PAD.encode(r#"{"alg":"HS512","typ":"JWT"}"#);
    let header_none = URL_SAFE_NO_PAD.encode(r#"{"alg":"none","typ":"JWT"}"#);
    for forged in [
        format!("{header_hs512}.{payload}.{signature}"),
        format!("{header_none}.{payload}."),
        format!("{header_none}.{payload}.{signature}"),
    ] {
        let (status, ..) = get(&router, "/me", Some(&format!("Bearer {forged}"))).await;
        assert_eq!(status, StatusCode::UNAUTHORIZED, "forged: {forged}");
    }
}

#[tokio::test]
async fn correctly_signed_token_without_expiry_is_rejected() {
    let (router, _) = router();
    let claims = serde_json::json!({"sub": "7", "role": "admin"});
    let token = jsonwebtoken::encode(
        &jsonwebtoken::Header::new(jsonwebtoken::Algorithm::HS256),
        &claims,
        &jsonwebtoken::EncodingKey::from_secret(SECRET),
    )
    .unwrap();
    let (status, ..) = get(&router, "/me", Some(&format!("Bearer {token}"))).await;
    assert_eq!(status, StatusCode::UNAUTHORIZED);
}

#[tokio::test]
async fn admin_routes_check_the_role() {
    let (router, auth) = router();
    let member = format!("Bearer {}", auth.issue("7", Role::Member, 60));
    let admin = format!("Bearer {}", auth.issue("1", Role::Admin, 60));

    let (status, challenge, body) = get(&router, "/admin/stats", Some(&member)).await;
    assert_eq!(status, StatusCode::FORBIDDEN);
    assert_eq!(challenge, None);
    assert!(body.contains(r#""code":"forbidden""#), "body: {body}");

    let (status, ..) = get(&router, "/admin/stats", Some(&admin)).await;
    assert_eq!(status, StatusCode::OK);

    // authentication is checked before authorization
    let (status, ..) = get(&router, "/admin/stats", None).await;
    assert_eq!(status, StatusCode::UNAUTHORIZED);
}
