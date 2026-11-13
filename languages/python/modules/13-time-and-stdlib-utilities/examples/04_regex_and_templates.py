"""Compiled regular expressions, named groups and string.Template."""

import re
import time
from string import Template

# Compile once at import time; name the groups.
TAG = re.compile(r"#(?P<name>[a-z][a-z0-9_-]{0,29})\b")
DURATION = re.compile(r"(?:(?P<h>\d+)h)?(?:(?P<m>\d+)m)?")

text = "Write docs #docs and fix #p1 bug, but not #Upper or ##"
print("tags:", [m["name"] for m in TAG.finditer(text)])

# fullmatch for validation: match() and search() accept partial input.
SLUG = re.compile(r"[a-z0-9]+(?:-[a-z0-9]+)*")
for candidate in ("write-docs", "Write-docs", "write--docs", "docs\n"):
    print(f"slug {candidate!r}:", bool(SLUG.fullmatch(candidate)))
print("match() is lenient:", bool(re.match(r"[a-z]+", "abc!!!")))

# sub with a function, and re.escape for user-supplied literals.
print("masked:", re.sub(r"[\w.]+@", lambda m: "*" * (len(m[0]) - 1) + "@", "ana.k@example.com"))
needle = "a+b"
print("escaped:", re.escape(needle), bool(re.search(re.escape(needle), "1 a+b 2")))

# string.Template: $-placeholders, no code execution, safe for user-edited text.
reminder = Template("Hi $name, '$title' is due on $due.")
print(reminder.substitute(name="Ana", title="Write docs", due="2030-01-31"))
print(reminder.safe_substitute(name="Ana"))


# Catastrophic backtracking: nested quantifiers on a non-matching input grow exponentially.
def seconds(pattern: re.Pattern[str], subject: str) -> float:
    started = time.perf_counter()
    pattern.match(subject)
    return time.perf_counter() - started


nested = seconds(re.compile(r"(a+)+$"), "a" * 22 + "!")
flat = seconds(re.compile(r"a+$"), "a" * 5000 + "!")
print("nested quantifier is slower despite 200x less input:", nested > flat)
