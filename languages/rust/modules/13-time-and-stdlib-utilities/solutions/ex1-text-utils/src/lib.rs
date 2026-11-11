//! Exercise 1: text utilities for the Task Tracker with `regex` and `minijinja`.

use std::sync::LazyLock;

use minijinja::{Environment, context};
use regex::Regex;

pub const MAX_TAGS: usize = 10;

/// `#` + a letter + up to 29 more of `[A-Za-z0-9_-]`, where the `#` starts the text or follows
/// a character that is not a letter, digit, `_` or `#`.
static TAG: LazyLock<Regex> = LazyLock::new(|| {
    Regex::new(r"(?:^|[^\w#])#([A-Za-z][A-Za-z0-9_-]{0,29})").expect("valid pattern")
});

/// Hashtags in `text`: lowercased, unique (first occurrence wins), at most [`MAX_TAGS`].
/// `"C#"` and `"a#b"` are not tags; `"#Bug #bug"` is one tag.
pub fn extract_tags(text: &str) -> Vec<String> {
    let mut tags: Vec<String> = Vec::new();
    for caps in TAG.captures_iter(text) {
        let tag = caps[1].to_ascii_lowercase();
        if !tags.contains(&tag) {
            tags.push(tag);
            if tags.len() == MAX_TAGS {
                break;
            }
        }
    }
    tags
}

/// URL-friendly slug: lowercase ASCII letters and digits, every other run of characters becomes one `-`,
/// no leading or trailing `-`. Example: `"Ship v2.0 -- now!"` -> `"ship-v2-0-now"`.
pub fn slugify(title: &str) -> String {
    static NON_ALNUM: LazyLock<Regex> =
        LazyLock::new(|| Regex::new(r"[^a-z0-9]+").expect("valid pattern"));
    let lower = title.to_ascii_lowercase();
    NON_ALNUM
        .replace_all(&lower, "-")
        .trim_matches('-')
        .to_string()
}

/// Plain-text reminder. `items` are `(title, already formatted due date)`.
///
/// ```text
/// Hi Ada, you have 2 task(s) due:
/// - Write docs (due Thu 31 Jan 2030)
/// - Ship it (due Fri 01 Feb 2030)
/// ```
/// Every line ends with `\n`. With no items the text is `Hi Ada, nothing is due.\n`.
pub fn render_reminder(name: &str, items: &[(String, String)]) -> Result<String, minijinja::Error> {
    const TEMPLATE: &str = "{% if items %}Hi {{ name }}, you have {{ items | length }} task(s) due:\n\
{% for item in items %}- {{ item.0 }} (due {{ item.1 }})\n{% endfor %}\
{% else %}Hi {{ name }}, nothing is due.\n{% endif %}";
    let mut env = Environment::new();
    // ".txt" keeps auto-escaping off, so "<" in a title stays "<"
    env.add_template("reminder.txt", TEMPLATE)?;
    env.get_template("reminder.txt")?
        .render(context! { name, items => items.to_vec() })
}

/// `<li class="task">{title}</li>` with the title HTML-escaped.
pub fn render_card_html(title: &str) -> Result<String, minijinja::Error> {
    let mut env = Environment::new();
    env.add_template("card.html", "<li class=\"task\">{{ title }}</li>")?;
    env.get_template("card.html")?.render(context! { title })
}
