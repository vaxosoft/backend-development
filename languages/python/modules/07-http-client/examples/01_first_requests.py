"""Making requests: query parameters, headers, JSON bodies and reading the response."""

import json

import httpx
from m07_helpers import Reply, Request, json_reply, serve


def echo(request: Request) -> Reply:
    path, _, query = request.path.partition("?")
    return json_reply(
        {
            "method": request.method,
            "path": path,
            "query": query,
            "accept": request.headers.get("accept"),
            "content_type": request.headers.get("content-type"),
            "body": request.body.decode(),
        }
    )


def main() -> None:
    with serve(echo) as base_url:
        # params= builds and percent-encodes the query string; a list repeats the key
        response = httpx.get(
            f"{base_url}/echo",
            params={"name": "Ada Lovelace", "tag": ["a", "b"]},
            headers={"Accept": "application/json"},
        )
        print(response.status_code, response.headers["content-type"])
        print(response.json())

        # json= serialises the dict and sets Content-Type: application/json
        response = httpx.post(f"{base_url}/echo", json={"title": "Write docs", "tags": ["docs"]})
        body = response.json()["body"]
        print(response.json()["content_type"], json.loads(body))

        # data= sends a form body; content= sends raw bytes you encoded yourself
        response = httpx.post(f"{base_url}/echo", data={"title": "Write docs"})
        print(response.json()["content_type"], response.json()["body"])
        response = httpx.put(f"{base_url}/echo", content=b"raw bytes")
        print(response.json()["method"], response.json()["body"])


if __name__ == "__main__":
    main()
