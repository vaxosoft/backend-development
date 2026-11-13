"""Read project metadata: pyproject.toml (the source) and importlib.metadata (what is installed)."""

import sys
import tomllib
from importlib.metadata import version
from pathlib import Path

pyproject = tomllib.loads(
    (Path(__file__).parent / "sample_pyproject.toml").read_text(encoding="utf-8")
)
project = pyproject["project"]

print("name:", project["name"])
print("version:", project["version"])
print("requires-python:", project["requires-python"])
print("dependencies:", project["dependencies"])
print("extras:", sorted(project["optional-dependencies"]))
print("entry points:", project["scripts"])
print("build backend:", pyproject["build-system"]["build-backend"])

# The same facts for an installed distribution (pinned in requirements.txt).
print("installed uvicorn:", version("uvicorn"))
print("python >= 3.14:", sys.version_info >= (3, 14))
print("running under -O:", not __debug__)
