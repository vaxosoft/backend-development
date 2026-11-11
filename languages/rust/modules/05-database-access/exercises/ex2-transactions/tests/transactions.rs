use m05_ex2_transactions::{
    count_tasks, create_task_with_tags, memory_pool, migrate, replace_tags, tags_of,
};
use sqlx::SqlitePool;

async fn db() -> SqlitePool {
    let pool = memory_pool().await;
    migrate(&pool).await.unwrap();
    pool
}

#[tokio::test]
async fn create_stores_task_and_tags() {
    let pool = db().await;
    let id = create_task_with_tags(&pool, "Write docs", &["docs", "p1"])
        .await
        .unwrap();
    assert_eq!(tags_of(&pool, id).await.unwrap(), ["docs", "p1"]);
    assert_eq!(count_tasks(&pool).await.unwrap(), 1);
}

#[tokio::test]
async fn failed_create_leaves_no_task_behind() {
    let pool = db().await;
    let err = create_task_with_tags(&pool, "Broken", &["a", "b", "a"])
        .await
        .unwrap_err();
    assert!(
        err.as_database_error()
            .is_some_and(|e| e.is_unique_violation()),
        "{err}"
    );
    assert_eq!(count_tasks(&pool).await.unwrap(), 0);
}

#[tokio::test]
async fn the_pool_is_usable_after_a_rolled_back_transaction() {
    let pool = db().await;
    create_task_with_tags(&pool, "Broken", &["a", "a"])
        .await
        .unwrap_err();
    let id = create_task_with_tags(&pool, "Fine", &["a"]).await.unwrap();
    assert_eq!(tags_of(&pool, id).await.unwrap(), ["a"]);
}

#[tokio::test]
async fn replace_swaps_the_whole_tag_set() {
    let pool = db().await;
    let id = create_task_with_tags(&pool, "T", &["old1", "old2"])
        .await
        .unwrap();
    replace_tags(&pool, id, &["new"]).await.unwrap();
    assert_eq!(tags_of(&pool, id).await.unwrap(), ["new"]);
    replace_tags(&pool, id, &[]).await.unwrap();
    assert!(tags_of(&pool, id).await.unwrap().is_empty());
}

#[tokio::test]
async fn failed_replace_keeps_the_old_tags() {
    let pool = db().await;
    let id = create_task_with_tags(&pool, "T", &["keep1", "keep2"])
        .await
        .unwrap();
    let err = replace_tags(&pool, id, &["x", "x"]).await.unwrap_err();
    assert!(err.as_database_error().is_some(), "{err}");
    assert_eq!(tags_of(&pool, id).await.unwrap(), ["keep1", "keep2"]);
}
