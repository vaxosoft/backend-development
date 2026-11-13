"""A tiny upstream to try the aggregator by hand: `python -m p4_api_aggregator.demo_upstream`.

Items 1-5 exist, 6 is slow (1 s), 7 always fails with 500, 8 is unknown (404).
"""

import asyncio
import os
from typing import Any

import uvicorn
from fastapi import FastAPI, HTTPException

app = FastAPI(title="demo upstream")


@app.get("/items/{item_id}")
async def get_item(item_id: int) -> dict[str, Any]:
    if item_id == 6:
        await asyncio.sleep(1)
    if item_id == 7:
        raise HTTPException(status_code=500, detail="boom")
    if item_id < 1 or item_id > 6:
        raise HTTPException(status_code=404, detail="no such item")
    return {"id": item_id, "name": f"item-{item_id}"}


def main() -> None:
    host, _, port = os.environ.get("UPSTREAM_ADDR", "127.0.0.1:9000").rpartition(":")
    uvicorn.run(app, host=host or "127.0.0.1", port=int(port))


if __name__ == "__main__":
    main()
