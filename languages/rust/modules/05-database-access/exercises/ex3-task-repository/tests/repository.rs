use chrono::NaiveDate;
use m05_ex3_task_repository::{NewTask, Status, TaskFilter, TaskRepo};

fn new_task(title: &str, status: Status, tags: &[&str]) -> NewTask {
    NewTask {
        title: title.into(),
        status,
        due_date: None,
        tags: tags.iter().map(|t| t.to_string()).collect(),
    }
}

async fn seeded() -> TaskRepo {
    let repo = TaskRepo::in_memory().await.unwrap();
    for (title, status, tags) in [
        ("write docs", Status::Todo, &["docs", "p1"][..]),
        ("review pr", Status::InProgress, &["code"]),
        ("ship v1", Status::Done, &["p1", "release"]),
        ("fix bug", Status::Todo, &["code", "p1"]),
    ] {
        repo.create(new_task(title, status, tags)).await.unwrap();
    }
    repo
}

fn titles(page: &m05_ex3_task_repository::Page) -> Vec<&str> {
    page.items.iter().map(|t| t.title.as_str()).collect()
}

#[tokio::test]
async fn create_and_get_round_trip() {
    let repo = TaskRepo::in_memory().await.unwrap();
    let created = repo
        .create(NewTask {
            title: "Plan".into(),
            status: Status::InProgress,
            due_date: NaiveDate::from_ymd_opt(2030, 1, 31),
            tags: vec!["b".into(), "a".into()],
        })
        .await
        .unwrap();
    let fetched = repo.get(created.id).await.unwrap().unwrap();
    assert_eq!(fetched.status, Status::InProgress);
    assert_eq!(fetched.due_date, NaiveDate::from_ymd_opt(2030, 1, 31));
    assert_eq!(fetched.tags, ["a", "b"]);
    assert!(repo.get(999).await.unwrap().is_none());
}

#[tokio::test]
async fn list_without_filters_is_newest_first() {
    let page = seeded().await.list(&TaskFilter::default()).await.unwrap();
    assert_eq!(
        titles(&page),
        ["fix bug", "ship v1", "review pr", "write docs"]
    );
    assert_eq!(page.total, 4);
    assert_eq!(page.items[0].tags, ["code", "p1"]);
}

#[tokio::test]
async fn filters_by_status_and_tag_and_both() {
    let repo = seeded().await;
    let by_status = TaskFilter {
        status: Some(Status::Todo),
        ..Default::default()
    };
    assert_eq!(
        titles(&repo.list(&by_status).await.unwrap()),
        ["fix bug", "write docs"]
    );

    let by_tag = TaskFilter {
        tag: Some("p1".into()),
        ..Default::default()
    };
    let page = repo.list(&by_tag).await.unwrap();
    assert_eq!(titles(&page), ["fix bug", "ship v1", "write docs"]);
    assert_eq!(page.total, 3);

    let both = TaskFilter {
        status: Some(Status::Todo),
        tag: Some("code".into()),
        ..Default::default()
    };
    assert_eq!(titles(&repo.list(&both).await.unwrap()), ["fix bug"]);
}

#[tokio::test]
async fn pagination_keeps_the_total_of_the_whole_match() {
    let repo = seeded().await;
    let filter = TaskFilter {
        limit: Some(2),
        offset: Some(1),
        ..Default::default()
    };
    let page = repo.list(&filter).await.unwrap();
    assert_eq!(titles(&page), ["ship v1", "review pr"]);
    assert_eq!(page.total, 4);

    let beyond = TaskFilter {
        offset: Some(10),
        ..Default::default()
    };
    let page = repo.list(&beyond).await.unwrap();
    assert!(page.items.is_empty());
    assert_eq!(page.total, 4);
}

#[tokio::test]
async fn hostile_tag_is_just_a_value() {
    let repo = seeded().await;
    let filter = TaskFilter {
        tag: Some("x' OR '1'='1".into()),
        ..Default::default()
    };
    let page = repo.list(&filter).await.unwrap();
    assert_eq!(page.total, 0);
    assert!(page.items.is_empty());
    assert_eq!(repo.list(&TaskFilter::default()).await.unwrap().total, 4);
}

#[tokio::test]
async fn update_status_returns_the_new_state_or_none() {
    let repo = seeded().await;
    let updated = repo.update_status(1, Status::Done).await.unwrap().unwrap();
    assert_eq!(
        (updated.title.as_str(), updated.status),
        ("write docs", Status::Done)
    );
    assert_eq!(repo.get(1).await.unwrap().unwrap().status, Status::Done);
    assert!(
        repo.update_status(999, Status::Done)
            .await
            .unwrap()
            .is_none()
    );
}

#[tokio::test]
async fn delete_reports_whether_a_row_existed_and_removes_tags() {
    let repo = seeded().await;
    assert!(repo.delete(1).await.unwrap());
    assert!(!repo.delete(1).await.unwrap());
    assert!(repo.get(1).await.unwrap().is_none());
    let orphans: i64 = sqlx::query_scalar("SELECT COUNT(*) FROM task_tags WHERE task_id = 1")
        .fetch_one(repo.pool())
        .await
        .unwrap();
    assert_eq!(orphans, 0);
}
