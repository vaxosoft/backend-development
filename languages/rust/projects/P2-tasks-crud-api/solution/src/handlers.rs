//! Route handlers: parse, validate, call the repository, shape the response.

use axum::Json;
use axum::extract::{Path, Query, State};
use axum::http::{StatusCode, header};
use axum::response::IntoResponse;

use crate::error::ApiError;
use crate::extract::ApiJson;
use crate::model::{NewTask, RawListQuery, RawPatch, RawTask, Task, TaskList};
use crate::repo::TaskRepo;
use crate::validate::{apply_patch, parse_id, parse_list, validate_new, validate_patch};

type ApiResult<T> = Result<T, ApiError>;

pub async fn create_task(
    State(repo): State<TaskRepo>,
    ApiJson(raw): ApiJson<RawTask>,
) -> ApiResult<impl IntoResponse> {
    let task = repo.create(&validate_new(raw)?).await?;
    let location = [(header::LOCATION, format!("/tasks/{}", task.id))];
    Ok((StatusCode::CREATED, location, Json(task)))
}

pub async fn list_tasks(
    State(repo): State<TaskRepo>,
    Query(raw): Query<RawListQuery>,
) -> ApiResult<Json<TaskList>> {
    let params = parse_list(raw)?;
    let page = repo.list(&params).await?;
    Ok(Json(TaskList {
        items: page.items,
        total: page.total,
        limit: params.limit,
        offset: params.offset,
    }))
}

pub async fn get_task(
    State(repo): State<TaskRepo>,
    Path(id): Path<String>,
) -> ApiResult<Json<Task>> {
    let task = repo.get(parse_id(&id)?).await?.ok_or(ApiError::NotFound)?;
    Ok(Json(task))
}

pub async fn replace_task(
    State(repo): State<TaskRepo>,
    Path(id): Path<String>,
    ApiJson(raw): ApiJson<RawTask>,
) -> ApiResult<Json<Task>> {
    let id = parse_id(&id)?;
    let new = validate_new(raw)?;
    let task = repo.replace(id, &new).await?.ok_or(ApiError::NotFound)?;
    Ok(Json(task))
}

pub async fn patch_task(
    State(repo): State<TaskRepo>,
    Path(id): Path<String>,
    ApiJson(raw): ApiJson<RawPatch>,
) -> ApiResult<Json<Task>> {
    let id = parse_id(&id)?;
    let patch = validate_patch(raw)?;
    let mut task = NewTask::from(repo.get(id).await?.ok_or(ApiError::NotFound)?);
    apply_patch(&mut task, patch);
    let task = repo.replace(id, &task).await?.ok_or(ApiError::NotFound)?;
    Ok(Json(task))
}

pub async fn delete_task(
    State(repo): State<TaskRepo>,
    Path(id): Path<String>,
) -> ApiResult<StatusCode> {
    if repo.delete(parse_id(&id)?).await? {
        Ok(StatusCode::NO_CONTENT)
    } else {
        Err(ApiError::NotFound)
    }
}
