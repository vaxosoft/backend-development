use m05_ex1_user_queries::{
    User, count_users, find_by_email, insert_user, list_users, memory_pool, migrate,
};
use sqlx::SqlitePool;

async fn db() -> SqlitePool {
    let pool = memory_pool().await;
    migrate(&pool).await.unwrap();
    pool
}

#[tokio::test]
async fn insert_returns_the_stored_user() {
    let pool = db().await;
    let ada = insert_user(&pool, "Ada", "ada@example.com").await.unwrap();
    let grace = insert_user(&pool, "Grace", "grace@example.com")
        .await
        .unwrap();
    assert_eq!(
        ada,
        User {
            id: 1,
            name: "Ada".into(),
            email: "ada@example.com".into()
        }
    );
    assert_eq!(grace.id, 2);
}

#[tokio::test]
async fn find_by_email_distinguishes_found_from_missing() {
    let pool = db().await;
    insert_user(&pool, "Ada", "ada@example.com").await.unwrap();
    let found = find_by_email(&pool, "ada@example.com").await.unwrap();
    assert_eq!(found.map(|u| u.name), Some("Ada".to_string()));
    assert_eq!(
        find_by_email(&pool, "nobody@example.com").await.unwrap(),
        None
    );
}

#[tokio::test]
async fn duplicate_email_is_a_unique_violation() {
    let pool = db().await;
    insert_user(&pool, "Ada", "ada@example.com").await.unwrap();
    let err = insert_user(&pool, "Impostor", "ada@example.com")
        .await
        .unwrap_err();
    let db_err = err.as_database_error().expect("a database error");
    assert!(db_err.is_unique_violation(), "{err}");
    assert_eq!(count_users(&pool).await.unwrap(), 1);
}

#[tokio::test]
async fn list_and_count_follow_insertion_order() {
    let pool = db().await;
    assert_eq!(count_users(&pool).await.unwrap(), 0);
    for (name, email) in [("B", "b@x.io"), ("A", "a@x.io"), ("C", "c@x.io")] {
        insert_user(&pool, name, email).await.unwrap();
    }
    let names: Vec<String> = list_users(&pool)
        .await
        .unwrap()
        .into_iter()
        .map(|u| u.name)
        .collect();
    assert_eq!(names, ["B", "A", "C"]);
    assert_eq!(count_users(&pool).await.unwrap(), 3);
}

#[tokio::test]
async fn values_are_bound_not_spliced_into_sql() {
    let pool = db().await;
    insert_user(&pool, "Ada", "ada@example.com").await.unwrap();
    let hostile = "x' OR '1'='1";
    assert_eq!(find_by_email(&pool, hostile).await.unwrap(), None);
    let stored = insert_user(&pool, "Robert'); DROP TABLE users;--", hostile)
        .await
        .unwrap();
    assert_eq!(stored.name, "Robert'); DROP TABLE users;--");
    assert_eq!(count_users(&pool).await.unwrap(), 2);
}
