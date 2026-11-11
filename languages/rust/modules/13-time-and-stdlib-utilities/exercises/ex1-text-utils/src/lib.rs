//! Exercise 1: text utilities for the Task Tracker with `regex` and `minijinja`.

use std::sync::LazyLock;

use regex::Regex;

pub const MAX_TAGS: usize = 10;

/// `#` + a letter + up to 29 more of `[A-Za-z0-9_-]`, where the `#` starts the text or follows
/// a character that is not a letter, digit, `_` or `#`.
#[allow(dead_code)] // used by your implementation
static TAG: LazyLock<Regex> = LazyLock::new(|| {
    Regex::new(r"(?:^|[^\w#])#([A-Za-z][A-Za-z0-9_-]{0,29})").expect("valid pattern")
});

/// Hashtags in `text`: lowercased, unique (first occurrence wins), at most [`MAX_TAGS`].
/// `"C#"` and `"a#b"` are not tags; `"#Bug #bug"` is one tag.
pub fn extract_tags(_text: &str) -> Vec<String> {
    todo!("use TAG.captures_iter, lowercase, de-duplicate in order, cap at MAX_TAGS")
}

/// URL-friendly slug: lowercase ASCII letters and digits, every other run of characters becomes one `-`,
/// no leading or trailing `-`. Example: `"Ship v2.0 -- now!"` -> `"ship-v2-0-now"`.
pub fn slugify(_title: &str) -> String {
    todo!("replace runs of non-alphanumeric characters with a single dash")
}

/// Plain-text reminder. `items` are `(title, already formatted due date)`.
///
/// ```text
/// Hi Ada, you have 2 task(s) due:
/// - Write docs (due Thu 31 Jan 2030)
/// - Ship it (due Fri 01 Feb 2030)
/// ```
/// Every line ends with `\n`. With no items the text is `Hi Ada, nothing is due.\n`.
pub fn render_reminder(
    _name: &str,
    _items: &[(String, String)],
) -> Result<String, minijinja::Error> {
    todo!("render a minijinja template; the plain-text template must NOT HTML-escape")
}

/// `<li class="task">{title}</li>` with the title HTML-escaped.
pub fn render_card_html(_title: &str) -> Result<String, minijinja::Error> {
    todo!("a template whose name ends in .html turns on auto-escaping")
}
