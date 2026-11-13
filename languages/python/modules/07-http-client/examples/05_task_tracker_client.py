"""A typed Task Tracker API client, tested without a network via httpx.MockTransport."""

import json
from collections.abc import Generator, Iterator
from dataclasses import dataclass

import httpx


@dataclass(slots=True, frozen=True)
class Task:
    id: int
    title: str
    status: str
    tags: list[str]


class ApiError(Exception):
    def __init__(self, status: int, code: str, message: str) -> None:
        super().__init__(f"{status} {code}: {message}")
        self.status = status
        self.code = code


class BearerAuth(httpx.Auth):
    """Adds the Authorization header to every request made through the client."""

    def __init__(self, token: str) -> None:
        self._token = token

    def auth_flow(self, request: httpx.Request) -> Generator[httpx.Request, httpx.Response]:
        request.headers["Authorization"] = f"Bearer {self._token}"
        yield request


class TaskTrackerClient:
    def __init__(self, client: httpx.Client) -> None:
        self._client = client

    def _json(self, response: httpx.Response) -> dict[str, object]:
        if response.is_success:
            body: dict[str, object] = response.json() if response.content else {}
            return body
        try:
            error = response.json()["error"]
            raise ApiError(response.status_code, error["code"], error["message"])
        except ValueError, KeyError, TypeError:
            raise ApiError(response.status_code, "unknown", response.reason_phrase) from None

    @staticmethod
    def _task(data: object) -> Task:
        assert isinstance(data, dict)
        return Task(data["id"], data["title"], data["status"], data["tags"])

    def create(self, title: str, tags: list[str] | None = None) -> Task:
        response = self._client.post("/tasks", json={"title": title, "tags": tags or []})
        return self._task(self._json(response))

    def get(self, task_id: int) -> Task:
        return self._task(self._json(self._client.get(f"/tasks/{task_id}")))

    def iter_tasks(self, page_size: int = 2) -> Iterator[Task]:
        offset = 0
        while True:
            page = self._json(
                self._client.get("/tasks", params={"limit": page_size, "offset": offset})
            )
            items = page["items"]
            assert isinstance(items, list)
            yield from (self._task(item) for item in items)
            offset += page_size
            if offset >= int(str(page["total"])):
                return


class FakeTaskTracker:
    """An in-memory stand-in for the real API; it is the `handler` of a MockTransport."""

    def __init__(self) -> None:
        self.tasks: list[dict[str, object]] = []
        self.seen_auth: set[str | None] = set()

    def __call__(self, request: httpx.Request) -> httpx.Response:
        self.seen_auth.add(request.headers.get("authorization"))
        if request.headers.get("authorization") != "Bearer s3cret":
            return self._error(401, "unauthorized", "missing or invalid token")
        path = request.url.path
        if request.method == "POST" and path == "/tasks":
            payload = json.loads(request.content)
            if not payload["title"].strip():
                return self._error(422, "validation_failed", "invalid task")
            task = {"id": len(self.tasks) + 1, "status": "todo", **payload}
            self.tasks.append(task)
            return httpx.Response(201, json=task, headers={"Location": f"/tasks/{task['id']}"})
        if request.method == "GET" and path == "/tasks":
            limit = int(request.url.params["limit"])
            offset = int(request.url.params["offset"])
            page = {
                "items": self.tasks[offset : offset + limit],
                "total": len(self.tasks),
                "limit": limit,
                "offset": offset,
            }
            return httpx.Response(200, json=page)
        if request.method == "GET" and path.startswith("/tasks/"):
            wanted = int(path.rsplit("/", 1)[1])
            for task in self.tasks:
                if task["id"] == wanted:
                    return httpx.Response(200, json=task)
        return self._error(404, "not_found", "task not found")

    @staticmethod
    def _error(status: int, code: str, message: str) -> httpx.Response:
        return httpx.Response(status, json={"error": {"code": code, "message": message}})


def main() -> None:
    fake = FakeTaskTracker()
    transport = httpx.MockTransport(fake)

    with httpx.Client(
        transport=transport, base_url="http://tracker.test", auth=BearerAuth("s3cret")
    ) as http:
        api = TaskTrackerClient(http)
        api.create("Write docs", ["docs"])
        api.create("Ship it")
        api.create("Celebrate", ["fun"])
        print(api.get(1))
        print([task.title for task in api.iter_tasks(page_size=2)])

        try:
            api.get(99)
        except ApiError as exc:
            print("ApiError:", exc, "| code =", exc.code)
        try:
            api.create("   ")
        except ApiError as exc:
            print("ApiError:", exc, "| code =", exc.code)

    with httpx.Client(transport=transport, base_url="http://tracker.test") as anonymous:
        try:
            TaskTrackerClient(anonymous).get(1)
        except ApiError as exc:
            print("ApiError:", exc)

    print("Authorization headers seen by the fake:", sorted(str(a) for a in fake.seen_auth))


if __name__ == "__main__":
    main()
