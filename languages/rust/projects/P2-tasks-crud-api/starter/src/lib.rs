//! P2: a JSON CRUD API for tasks, backed by SQLite.

pub mod error;
pub mod extract;
pub mod handlers;
pub mod model;
pub mod repo;
pub mod validate;

use axum::Router;
use axum::routing::get;

use crate::repo::TaskRepo;

#[derive(Debug, Clone)]
pub struct Config {
    /// For example `sqlite::memory:` or `sqlite://tasks.db?mode=rwc`.
    pub database_url: String,
}

impl Default for Config {
    fn default() -> Self {
        Self {
            database_url: "sqlite::memory:".to_owned(),
        }
    }
}

/// Opens the database (creating the schema) and builds the router.
pub async fn build_app(config: &Config) -> Result<Router, sqlx::Error> {
    let repo = TaskRepo::connect(&config.database_url).await?;
    Ok(Router::new()
        .route(
            "/tasks",
            get(handlers::list_tasks).post(handlers::create_task),
        )
        .route(
            "/tasks/{id}",
            get(handlers::get_task)
                .put(handlers::replace_task)
                .patch(handlers::patch_task)
                .delete(handlers::delete_task),
        )
        .with_state(repo))
}
