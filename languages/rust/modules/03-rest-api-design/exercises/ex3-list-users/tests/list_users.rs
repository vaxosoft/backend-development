use axum::body::Body;
use axum::http::{Request, StatusCode};
use http_body_util::BodyExt;
use m03_ex3_list_users::{QueryError, User, UserQuery, list_users, router};
use serde_json::Value;
use tower::ServiceExt;

fn users() -> Vec<User> {
    [
        (1, "Grace Hopper", "grace@navy.example"),
        (2, "alan turing", "alan@bletchley.example"),
        (3, "Ada Lovelace", "ada@analytical.example"),
        (4, "Barbara Liskov", "barbara@mit.example"),
        (5, "Ada Yonath", "yonath@weizmann.example"),
    ]
    .into_iter()
    .map(|(id, name, email)| User {
        id,
        name: name.to_string(),
        email: email.to_string(),
    })
    .collect()
}

fn query(
    q: Option<&str>,
    sort: Option<&str>,
    limit: Option<usize>,
    offset: Option<usize>,
) -> UserQuery {
    UserQuery {
        q: q.map(String::from),
        sort: sort.map(String::from),
        limit,
        offset,
    }
}

fn ids(page: &m03_ex3_list_users::Page<User>) -> Vec<u32> {
    page.items.iter().map(|u| u.id).collect()
}

#[test]
fn defaults_are_id_order_limit_20_offset_0() {
    let page = list_users(&users(), &UserQuery::default()).unwrap();
    assert_eq!(ids(&page), [1, 2, 3, 4, 5]);
    assert_eq!((page.total, page.limit, page.offset), (5, 20, 0));
}

#[test]
fn q_matches_name_or_email_ignoring_case() {
    let by_name = list_users(&users(), &query(Some("ADA"), None, None, None)).unwrap();
    assert_eq!(ids(&by_name), [3, 5]);
    let by_email = list_users(&users(), &query(Some("navy"), None, None, None)).unwrap();
    assert_eq!(ids(&by_email), [1]);
    let empty_q = list_users(&users(), &query(Some(""), None, None, None)).unwrap();
    assert_eq!(empty_q.total, 5);
}

#[test]
fn sorting_by_name_ignores_case() {
    let asc = list_users(&users(), &query(None, Some("name"), None, None)).unwrap();
    assert_eq!(ids(&asc), [3, 5, 2, 4, 1]);
    let desc = list_users(&users(), &query(None, Some("-name"), None, None)).unwrap();
    assert_eq!(ids(&desc), [1, 4, 2, 5, 3]);
    let by_id_desc = list_users(&users(), &query(None, Some("-id"), None, None)).unwrap();
    assert_eq!(ids(&by_id_desc), [5, 4, 3, 2, 1]);
}

#[test]
fn equal_names_fall_back_to_ascending_id_in_both_directions() {
    let twins: Vec<User> = [(7, "Sam"), (3, "sam"), (5, "SAM")]
        .into_iter()
        .map(|(id, name)| User {
            id,
            name: name.to_string(),
            email: format!("{id}@example.com"),
        })
        .collect();
    let asc = list_users(&twins, &query(None, Some("name"), None, None)).unwrap();
    assert_eq!(ids(&asc), [3, 5, 7]);
    let desc = list_users(&twins, &query(None, Some("-name"), None, None)).unwrap();
    assert_eq!(ids(&desc), [3, 5, 7]);
}

#[test]
fn unknown_sort_is_an_error() {
    assert_eq!(
        list_users(&users(), &query(None, Some("email"), None, None)),
        Err(QueryError::BadSort("email".into()))
    );
}

#[test]
fn pagination_slices_after_filtering_and_reports_the_filtered_total() {
    let page = list_users(&users(), &query(Some("a"), Some("id"), Some(2), Some(1))).unwrap();
    // every user has an "a" somewhere; skip one item, then take two
    assert_eq!(ids(&page), [2, 3]);
    assert_eq!((page.total, page.limit, page.offset), (5, 2, 1));
}

#[test]
fn limit_is_clamped_and_offset_past_the_end_is_empty() {
    let big = list_users(&users(), &query(None, None, Some(1000), None)).unwrap();
    assert_eq!(big.limit, 100);
    let zero = list_users(&users(), &query(None, None, Some(0), None)).unwrap();
    assert_eq!((zero.limit, zero.items.len()), (1, 1));
    let past = list_users(&users(), &query(None, None, None, Some(50))).unwrap();
    assert!(past.items.is_empty());
    assert_eq!((past.total, past.offset), (5, 50));
}

async fn get(uri: &str) -> (StatusCode, Value) {
    let response = router(users())
        .oneshot(Request::get(uri).body(Body::empty()).unwrap())
        .await
        .unwrap();
    let status = response.status();
    let bytes = response.into_body().collect().await.unwrap().to_bytes();
    (
        status,
        serde_json::from_slice(&bytes).unwrap_or(Value::Null),
    )
}

#[tokio::test]
async fn http_returns_the_page_envelope() {
    let (status, body) = get("/users?q=ada&sort=-name&limit=1").await;
    assert_eq!(status, StatusCode::OK);
    assert_eq!(body["total"], 2);
    assert_eq!(body["limit"], 1);
    assert_eq!(body["offset"], 0);
    assert_eq!(body["items"][0]["name"], "Ada Yonath");
}

#[tokio::test]
async fn http_rejects_bad_sort_and_bad_numbers_with_400() {
    let (status, body) = get("/users?sort=age").await;
    assert_eq!(status, StatusCode::BAD_REQUEST);
    assert_eq!(body["error"]["code"], "bad_request");
    assert_eq!(get("/users?limit=abc").await.0, StatusCode::BAD_REQUEST);
    assert_eq!(get("/users?offset=-1").await.0, StatusCode::BAD_REQUEST);
}
