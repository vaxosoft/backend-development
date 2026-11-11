//! Route handlers. All TODO stubs answer 501.

use axum::http::StatusCode;

// TODO: `State<TaskRepo>` + `ApiJson<RawTask>` -> validate -> `repo.create`
// -> 201, `Location: /tasks/{id}` and the task as JSON.
pub async fn create_task() -> StatusCode {
    StatusCode::NOT_IMPLEMENTED
}

// TODO: `Query<RawListQuery>` -> `parse_list` -> `repo.list` -> `TaskList`.
pub async fn list_tasks() -> StatusCode {
    StatusCode::NOT_IMPLEMENTED
}

// TODO: 200 with the task, 404 when unknown.
pub async fn get_task() -> StatusCode {
    StatusCode::NOT_IMPLEMENTED
}

// TODO: full replace; 200 with the new task, 404 when unknown.
pub async fn replace_task() -> StatusCode {
    StatusCode::NOT_IMPLEMENTED
}

// TODO: load, `apply_patch`, `repo.replace`; 200 with the updated task.
pub async fn patch_task() -> StatusCode {
    StatusCode::NOT_IMPLEMENTED
}

// TODO: 204, or 404 when unknown.
pub async fn delete_task() -> StatusCode {
    StatusCode::NOT_IMPLEMENTED
}
