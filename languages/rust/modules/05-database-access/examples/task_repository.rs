//! Task Tracker repository: migrations, a repository trait, dynamic filters, pagination.

use chrono::{NaiveDate, Utc};
use sqlx::sqlite::SqlitePoolOptions;
use sqlx::{FromRow, QueryBuilder, Sqlite, SqlitePool};

/// Embeds `migrations/*.sql` into the binary at compile time.
static MIGRATOR: sqlx::migrate::Migrator = sqlx::migrate!("./migrations");

#[derive(Debug, Clone, Copy, PartialEq, Eq, sqlx::Type)]
#[sqlx(type_name = "TEXT", rename_all = "snake_case")]
enum Status {
    Todo,
    InProgress,
    Done,
}

struct NewTask {
    title: String,
    status: Status,
    due_date: Option<NaiveDate>,
    tags: Vec<String>,
}

struct Task {
    id: i64,
    title: String,
    status: Status,
    due_date: Option<NaiveDate>,
    tags: Vec<String>,
}

#[derive(FromRow)]
struct TaskRecord {
    id: i64,
    title: String,
    status: Status,
    due_date: Option<NaiveDate>,
}

#[derive(Default)]
struct TaskFilter {
    status: Option<Status>,
    tag: Option<String>,
    limit: Option<i64>,
    offset: Option<i64>,
}

struct Page {
    items: Vec<Task>,
    total: i64,
}

#[derive(Debug, thiserror::Error)]
enum RepoError {
    #[error("task {0} not found")]
    NotFound(i64),
    #[error(transparent)]
    Db(#[from] sqlx::Error),
}

/// The rest of the program depends on this trait, not on SQLx: a fake can replace it in tests.
trait TaskRepository {
    async fn create(&self, new: NewTask) -> Result<Task, RepoError>;
    async fn get(&self, id: i64) -> Result<Option<Task>, RepoError>;
    async fn list(&self, filter: &TaskFilter) -> Result<Page, RepoError>;
    async fn set_status(&self, id: i64, status: Status) -> Result<Task, RepoError>;
    async fn delete(&self, id: i64) -> Result<(), RepoError>;
}

struct SqliteTaskRepository {
    pool: SqlitePool,
}

impl SqliteTaskRepository {
    async fn with_tags(&self, record: TaskRecord) -> Result<Task, sqlx::Error> {
        let tags = sqlx::query_scalar("SELECT tag FROM task_tags WHERE task_id = ? ORDER BY tag")
            .bind(record.id)
            .fetch_all(&self.pool)
            .await?;
        Ok(Task {
            id: record.id,
            title: record.title,
            status: record.status,
            due_date: record.due_date,
            tags,
        })
    }
}

fn push_filter(builder: &mut QueryBuilder<Sqlite>, filter: &TaskFilter) {
    if let Some(status) = filter.status {
        builder.push(" AND status = ").push_bind(status);
    }
    if let Some(tag) = &filter.tag {
        builder
            .push(" AND EXISTS (SELECT 1 FROM task_tags WHERE task_id = tasks.id AND tag = ")
            .push_bind(tag.clone())
            .push(")");
    }
}

impl TaskRepository for SqliteTaskRepository {
    async fn create(&self, new: NewTask) -> Result<Task, RepoError> {
        let mut tx = self.pool.begin().await?;
        let id: i64 = sqlx::query_scalar(
            "INSERT INTO tasks (title, status, due_date, created_at) VALUES (?, ?, ?, ?) RETURNING id",
        )
        .bind(&new.title)
        .bind(new.status)
        .bind(new.due_date)
        .bind(Utc::now())
        .fetch_one(&mut *tx)
        .await?;
        for tag in &new.tags {
            sqlx::query("INSERT INTO task_tags (task_id, tag) VALUES (?, ?)")
                .bind(id)
                .bind(tag)
                .execute(&mut *tx)
                .await?;
        }
        tx.commit().await?;
        Ok(Task {
            id,
            title: new.title,
            status: new.status,
            due_date: new.due_date,
            tags: new.tags,
        })
    }

    async fn get(&self, id: i64) -> Result<Option<Task>, RepoError> {
        let record: Option<TaskRecord> =
            sqlx::query_as("SELECT id, title, status, due_date FROM tasks WHERE id = ?")
                .bind(id)
                .fetch_optional(&self.pool)
                .await?;
        match record {
            Some(record) => Ok(Some(self.with_tags(record).await?)),
            None => Ok(None),
        }
    }

    async fn list(&self, filter: &TaskFilter) -> Result<Page, RepoError> {
        let limit = filter.limit.unwrap_or(20).clamp(1, 100);
        let offset = filter.offset.unwrap_or(0).max(0);

        let mut select =
            QueryBuilder::new("SELECT id, title, status, due_date FROM tasks WHERE 1 = 1");
        push_filter(&mut select, filter);
        select
            .push(" ORDER BY id DESC LIMIT ")
            .push_bind(limit)
            .push(" OFFSET ")
            .push_bind(offset);
        let records: Vec<TaskRecord> = select.build_query_as().fetch_all(&self.pool).await?;

        let mut items = Vec::with_capacity(records.len());
        for record in records {
            items.push(self.with_tags(record).await?);
        }

        let mut count = QueryBuilder::new("SELECT COUNT(*) FROM tasks WHERE 1 = 1");
        push_filter(&mut count, filter);
        let total = count.build_query_scalar().fetch_one(&self.pool).await?;
        Ok(Page { items, total })
    }

    async fn set_status(&self, id: i64, status: Status) -> Result<Task, RepoError> {
        let result = sqlx::query("UPDATE tasks SET status = ? WHERE id = ?")
            .bind(status)
            .bind(id)
            .execute(&self.pool)
            .await?;
        if result.rows_affected() == 0 {
            return Err(RepoError::NotFound(id));
        }
        self.get(id).await?.ok_or(RepoError::NotFound(id))
    }

    async fn delete(&self, id: i64) -> Result<(), RepoError> {
        let result = sqlx::query("DELETE FROM tasks WHERE id = ?")
            .bind(id)
            .execute(&self.pool)
            .await?;
        if result.rows_affected() == 0 {
            return Err(RepoError::NotFound(id));
        }
        Ok(())
    }
}

fn describe(task: &Task) -> String {
    let due = task.due_date.map_or("-".to_string(), |d| d.to_string());
    format!(
        "#{} {:<12} {:?} due={due} tags={:?}",
        task.id, task.title, task.status, task.tags
    )
}

async fn print_page(
    repo: &impl TaskRepository,
    label: &str,
    filter: TaskFilter,
) -> Result<(), RepoError> {
    let page = repo.list(&filter).await?;
    println!("{label}: total={}", page.total);
    for task in &page.items {
        println!("  {}", describe(task));
    }
    Ok(())
}

#[tokio::main]
async fn main() -> Result<(), RepoError> {
    let pool = SqlitePoolOptions::new()
        .max_connections(1)
        .connect("sqlite::memory:")
        .await?;

    // Migrations are recorded in `_sqlx_migrations`; running them again applies nothing.
    MIGRATOR.run(&pool).await.map_err(sqlx::Error::from)?;
    MIGRATOR.run(&pool).await.map_err(sqlx::Error::from)?;
    let applied: Vec<(i64, String)> =
        sqlx::query_as("SELECT version, description FROM _sqlx_migrations ORDER BY version")
            .fetch_all(&pool)
            .await?;
    println!("migrations: {applied:?}");

    let repo = SqliteTaskRepository { pool };
    let seeds = [
        (
            "Write docs",
            Status::Todo,
            NaiveDate::from_ymd_opt(2030, 1, 31),
            vec!["docs", "p1"],
        ),
        ("Review PR", Status::InProgress, None, vec!["code"]),
        (
            "Ship v1",
            Status::Done,
            NaiveDate::from_ymd_opt(2030, 2, 1),
            vec!["p1", "release"],
        ),
        ("Fix bug", Status::Todo, None, vec!["code", "p1"]),
    ];
    for (title, status, due_date, tags) in seeds {
        let tags = tags.into_iter().map(String::from).collect();
        repo.create(NewTask {
            title: title.into(),
            status,
            due_date,
            tags,
        })
        .await?;
    }

    print_page(&repo, "all", TaskFilter::default()).await?;
    print_page(
        &repo,
        "tag=p1 status=todo",
        TaskFilter {
            status: Some(Status::Todo),
            tag: Some("p1".into()),
            ..Default::default()
        },
    )
    .await?;
    print_page(
        &repo,
        "page 2 of size 2",
        TaskFilter {
            limit: Some(2),
            offset: Some(2),
            ..Default::default()
        },
    )
    .await?;

    let updated = repo.set_status(2, Status::Done).await?;
    println!("updated: {}", describe(&updated));
    repo.delete(1).await?;
    println!("after delete: {:?}", repo.get(1).await?.map(|t| t.id));
    match repo.set_status(99, Status::Done).await {
        Err(e) => println!("error: {e}"),
        Ok(_) => unreachable!("task 99 does not exist"),
    }
    Ok(())
}
