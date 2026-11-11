#!/usr/bin/env python3
"""Verify a language track.

    scripts/verify.py <go|rust|csharp> [name-filter]

Works on a throw-away copy of languages/<lang> (fast Linux filesystem, nothing
in the repo is modified). For every exercise and capstone project it checks:

  1. starter tests FAIL (and fail on assertions/panics, not on compile errors)
  2. after overlaying solutions/ (or solution/) the same tests PASS

Then, with every solution applied, it runs formatter, linter and builds all
examples. Exit code is non-zero if any check fails.

Layout contract (see framework/README.md, "Repository mechanics"):
  modules/NN-topic/examples/...        runnable examples
  modules/NN-topic/exercises/...       starter + tests
  modules/NN-topic/solutions/...       files overlaid on exercises/
  projects/PN-name/{starter,solution,tests}/
(Go uses _solutions/ and _solution/ so the Go tool ignores them.)
"""
import os
import re
import shutil
import subprocess
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
LANG = sys.argv[1] if len(sys.argv) > 1 else sys.exit(__doc__)
FILTER = sys.argv[2] if len(sys.argv) > 2 else ""
SRC = ROOT / "languages" / LANG
WORK = Path(os.environ.get("VERIFY_WORKDIR") or f"/tmp/verify-{LANG}-{os.getpid()}")
SOL = "_solution" if LANG == "go" else "solution"
SOLS = SOL + "s"
IGNORE = shutil.ignore_patterns("bin", "obj", "target", ".git", "node_modules", "*.db", "*.db-*")

env = os.environ.copy()
env["PATH"] = f"{Path.home()}/.dotnet-sdk:{Path.home()}/.local/go/bin:{Path.home()}/.cargo/bin:" + env["PATH"]
env.update(DOTNET_NOLOGO="1", DOTNET_CLI_TELEMETRY_OPTOUT="1", CARGO_TARGET_DIR="/tmp/target-verify",
           CARGO_TERM_COLOR="never", NO_COLOR="1")

failures: list[str] = []
results: list[tuple[str, str]] = []


def run(cmd, cwd, timeout=900):
    p = subprocess.run(cmd, cwd=cwd, env=env, capture_output=True, text=True, timeout=timeout)
    return p.returncode, p.stdout + p.stderr


def record(name, ok, detail=""):
    results.append((name, "ok" if ok else "FAIL"))
    if not ok:
        failures.append(name)
        print(f"FAIL {name}\n{detail[-3000:]}\n")
    else:
        print(f"ok   {name}")


def overlay(solution: Path, target: Path):
    for f in solution.rglob("*"):
        if f.is_file() and not any(p in ("bin", "obj", "target") for p in f.parts):
            dest = target / f.relative_to(solution)
            dest.parent.mkdir(parents=True, exist_ok=True)
            shutil.copy(f, dest)


def restore(solution: Path, target: Path, orig_target: Path):
    for f in solution.rglob("*"):
        if f.is_file():
            rel = f.relative_to(solution)
            o = orig_target / rel
            if o.exists():
                shutil.copy(o, target / rel)
            else:
                (target / rel).unlink(missing_ok=True)


# Per-language adapters -------------------------------------------------------

def compile_error(out: str) -> bool:
    return bool(re.search(r"\[build failed\]|\[setup failed\]|error\[E\d+\]|could not compile|error CS\d+|Build FAILED", out))


def test_cmd_for(unit: dict):
    """Return (command, cwd) to test a unit (exercise or project)."""
    kind, path = unit["kind"], unit.get("path")
    if LANG == "go":
        if kind == "exercise":
            return ["go", "test", "-count=1", "./" + str(path.relative_to(WORK / "")) + "/..."], WORK
        return ["go", "test", "-count=1", "./" + str(path.relative_to(WORK)) + "/..."], WORK
    if LANG == "rust":
        pkg = re.search(r'^name\s*=\s*"([^"]+)"', (unit["crate"] / "Cargo.toml").read_text(), re.M).group(1)
        return ["cargo", "test", "-p", pkg], WORK
    if LANG == "csharp":
        proj = unit["proj"]
        cmd = ["dotnet", "test", str(proj)]
        if unit.get("filter"):
            cmd += ["--filter", unit["filter"]]
        return cmd, WORK
    raise SystemExit(f"unknown language {LANG}")


def discover():
    units = []
    for mod in sorted((WORK / "modules").glob("[0-9][0-9]-*")):
        if mod.name.startswith("00-"):
            continue
        ex = mod / "exercises"
        sol = mod / SOLS
        if not ex.is_dir():
            continue
        if LANG == "csharp":
            projs = list(ex.glob("*.csproj"))
            if not projs:
                continue
            nums = sorted({m.group(1) for f in ex.glob("Ex*.cs") if (m := re.match(r"Ex(\d+)", f.name))})
            for n in nums:
                units.append(dict(kind="exercise", name=f"{mod.name}/ex{n}", proj=projs[0], filter=f"FullyQualifiedName~.Ex{n}_",
                                  overlay=[(sol, ex)], files=[f for f in sol.glob(f"Ex{n}*.cs")]))
        else:
            for e in sorted(p for p in ex.iterdir() if p.is_dir()):
                s = sol / e.name
                units.append(dict(kind="exercise", name=f"{mod.name}/{e.name}", path=e, crate=e, overlay=[(s, e)]))
    for proj in sorted((WORK / "projects").glob("P[0-9]*-*")):
        if proj.name.startswith("P0-"):
            continue
        st, so = proj / "starter", proj / SOL
        if not st.is_dir():
            continue
        u = dict(kind="project", name=f"projects/{proj.name}", path=proj, crate=st, overlay=[(so, st)])
        if LANG == "csharp":
            tp = list((proj / "tests").glob("*.csproj"))
            u["proj"] = tp[0] if tp else None
        units.append(u)
    return [u for u in units if FILTER in u["name"]]


def main():
    if WORK.exists():
        shutil.rmtree(WORK)
    shutil.copytree(SRC, WORK, ignore=IGNORE, copy_function=shutil.copy)
    units = discover()
    print(f"Verifying {LANG}: {len(units)} units\n")

    for u in units:
        if u.get("proj") is None and LANG == "csharp":
            record(u["name"], False, "no test project found")
            continue
        overlays = u["overlay"]
        if not all(s.exists() and any(s.rglob("*")) for s, _ in overlays):
            record(u["name"], False, "missing or empty solution")
            continue
        cmd, cwd = test_cmd_for(u)
        rc, out = run(cmd, cwd)
        if rc == 0:
            record(f"{u['name']} [starter must fail]", False, "starter tests already pass")
            continue
        if compile_error(out):
            record(f"{u['name']} [starter must fail]", False, "starter does not compile:\n" + out)
            continue
        for s, t in overlays:
            overlay(s, t)
        rc, out = run(cmd, cwd)
        record(f"{u['name']} [starter fails, solution passes]", rc == 0, out)
        for s, t in overlays:
            restore(s, t, SRC / t.relative_to(WORK))

    # Solved tree: apply everything, then lint/format/build.
    for u in units:
        for s, t in u["overlay"]:
            if s.exists():
                overlay(s, t)
    checks = {
        "go": [("gofmt", ["bash", "-c", "test -z \"$(gofmt -l .)\" || (gofmt -l .; exit 1)"]),
               ("go vet", ["go", "vet", "./..."]),
               ("go build", ["go", "build", "./..."])],
        "rust": [("cargo fmt", ["cargo", "fmt", "--all", "--", "--check"]),
                 ("cargo clippy", ["cargo", "clippy", "--workspace", "--all-targets", "--", "-D", "warnings"]),
                 ("cargo build examples", ["cargo", "build", "--workspace", "--examples", "--bins"])],
        "csharp": [("dotnet build all", ["bash", "-c",
                    "set -e; for p in $(find modules projects -name '*.csproj' | sort); do dotnet build \"$p\" --nologo -v q || exit 1; done"]),
                   ("dotnet format", ["bash", "-c",
                    "set -e; for p in $(find modules projects -name '*.csproj' ! -path '*/00-*' | sort); do dotnet format whitespace \"$p\" --verify-no-changes || exit 1; done"])],
    }[LANG]
    if not FILTER:
        for name, cmd in checks:
            rc, out = run(cmd, WORK, timeout=3000)
            record(name, rc == 0, out)

    if not os.environ.get("VERIFY_KEEP"):
        shutil.rmtree(WORK, ignore_errors=True)
    print(f"\n{len(results) - len(failures)}/{len(results)} checks passed")
    sys.exit(1 if failures else 0)


main()
