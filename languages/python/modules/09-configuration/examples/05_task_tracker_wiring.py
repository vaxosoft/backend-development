"""Wiring: build settings once, pass them down, and override them in tests."""

import asyncio
import secrets
from dataclasses import dataclass, field
from typing import Annotated

import httpx
from fastapi import Depends, FastAPI, Header, HTTPException, Request
from m09_task_settings import Settings
from pydantic import BaseModel


class NewTask(BaseModel):
    title: str


@dataclass(slots=True)
class TaskRepository:
    titles: list[str] = field(default_factory=list)

    def add(self, title: str) -> int:
        self.titles.append(title)
        return len(self.titles)


def create_app(settings: Settings) -> FastAPI:
    """The composition root: the only place that decides which pieces are wired together."""
    app = FastAPI(title="Task Tracker", debug=settings.debug)
    app.state.settings = settings
    app.state.repo = TaskRepository()

    def get_settings(request: Request) -> Settings:
        current: Settings = request.app.state.settings
        return current

    def get_repo(request: Request) -> TaskRepository:
        repo: TaskRepository = request.app.state.repo
        return repo

    def require_token(
        settings: Annotated[Settings, Depends(get_settings)],
        x_api_token: Annotated[str, Header()] = "",
    ) -> None:
        if not secrets.compare_digest(x_api_token, settings.api_token.get_secret_value()):
            raise HTTPException(status_code=401, detail="unauthorized")

    @app.get("/config")
    def show_config(settings: Annotated[Settings, Depends(get_settings)]) -> dict[str, object]:
        # in JSON mode a SecretStr is dumped masked, never revealed
        return settings.model_dump(mode="json")

    @app.post("/tasks", status_code=201, dependencies=[Depends(require_token)])
    def add_task(
        body: NewTask, repo: Annotated[TaskRepository, Depends(get_repo)]
    ) -> dict[str, object]:
        return {"id": repo.add(body.title), "title": body.title}

    return app


def client_for(app: FastAPI) -> httpx.AsyncClient:
    return httpx.AsyncClient(transport=httpx.ASGITransport(app=app), base_url="http://tracker.test")


async def main() -> None:
    # Settings(...) values here stand in for what the environment would provide
    production = Settings(port=8080, api_token="prod-secret", log_level="warn")  # type: ignore[arg-type]
    testing = Settings(port=1, api_token="test-token", database_url="sqlite://", debug=True)  # type: ignore[arg-type]
    body = {"title": "Write docs"}

    async with client_for(create_app(production)) as client:
        print((await client.get("/config")).json())
        print((await client.post("/tasks", json=body)).status_code)
        response = await client.post("/tasks", json=body, headers={"X-API-Token": "prod-secret"})
        print(response.status_code, response.json())

    # A second app with different settings in the same process: no global state to reset
    async with client_for(create_app(testing)) as client:
        print((await client.get("/config")).json()["debug"])
        response = await client.post("/tasks", json=body, headers={"X-API-Token": "prod-secret"})
        print("production token on the testing app:", response.status_code)


if __name__ == "__main__":
    asyncio.run(main())
