"""Other formats: CSV, XML, TOML, YAML and form-encoded data from the standard toolbox."""

import csv
import io
import tomllib
import xml.etree.ElementTree as ET
from urllib.parse import parse_qs, urlencode

import yaml

# --- CSV: quoting is handled for you, but only if you let the csv module write the rows.
buffer = io.StringIO(newline="")
writer = csv.DictWriter(buffer, fieldnames=["id", "title", "tags"], lineterminator="\n")
writer.writeheader()
writer.writerow({"id": 1, "title": 'Say "hi", then leave', "tags": "docs;p1"})
writer.writerow({"id": 2, "title": "Plain", "tags": ""})
print(buffer.getvalue(), end="")
for row in csv.DictReader(io.StringIO(buffer.getvalue(), newline="")):
    print(row)  # every CSV value is a string; convert it yourself

# --- XML: build and parse trees. Never hand-concatenate strings: the library escapes for you.
root = ET.Element("task", id="1")
ET.SubElement(root, "title").text = "Fish & chips <urgent>"
tags = ET.SubElement(root, "tags")
for tag in ("food", "p1"):
    ET.SubElement(tags, "tag").text = tag
xml_text = ET.tostring(root, encoding="unicode")
print(xml_text)
parsed = ET.fromstring(xml_text)
print(parsed.get("id"), parsed.findtext("title"), [t.text for t in parsed.iterfind("tags/tag")])

# --- TOML: read-only in the stdlib (tomllib). Types survive: ints, bools, arrays.
config = tomllib.loads(
    'title = "Task Tracker"\nport = 8080\ndebug = false\n[db]\nurl = "tasks.db"\n'
)
print(config)

# --- YAML: safe_load only builds plain data. yaml.load with an unsafe loader can construct objects.
print(yaml.safe_load("title: Write docs\ntags: [docs, p1]\ndue_date: 2030-01-31\nenabled: no\n"))
try:
    yaml.safe_load("!!python/object/apply:os.system ['echo pwned']")
except yaml.YAMLError as err:
    print("safe_load refused:", type(err).__name__)

# --- application/x-www-form-urlencoded: values are lists because keys may repeat.
body = urlencode({"title": "Buy milk & eggs", "tag": ["food", "p1"]}, doseq=True)
print(body)
print(parse_qs(body))
