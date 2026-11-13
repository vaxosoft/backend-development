"""Exercise 3: PATCH semantics - absent, null and present are three different things."""

from typing import Any

from pydantic import BaseModel, ConfigDict


class PatchError(Exception):
    def __init__(self, fields: dict[str, str]) -> None:
        super().__init__(f"invalid patch: {fields}")
        self.fields = fields


class TaskPatch(BaseModel):
    """Declare the patchable fields here. Add whatever validation the rules below need."""

    model_config = ConfigDict(extra="forbid")


def apply_patch(task: dict[str, Any], body: str) -> dict[str, Any]:
    """Return a NEW dict: `task` with the JSON object in `body` applied. Never mutate `task`.

    - patchable: title (str), status (todo|in_progress|done), due_date (YYYY-MM-DD or null),
      tags (list of str, no duplicates), owner_id (int or null)
    - absent key -> unchanged; `null` clears `due_date` and `owner_id`
    - `null` for title, status or tags is an error; so are unknown keys (including `id`)
    - title is trimmed and must be 1-200 characters
    - values are stored in wire form: status as its string, due_date as "YYYY-MM-DD"
    - on any problem raise PatchError(fields) with one entry per offending field;
      malformed JSON is PatchError({"body": "malformed JSON"})
    """
    raise NotImplementedError
