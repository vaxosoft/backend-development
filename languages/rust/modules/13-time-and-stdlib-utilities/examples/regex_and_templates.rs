//! Regular expressions (compile once) and text templates (minijinja with HTML auto-escaping).

use std::collections::HashSet;
use std::sync::LazyLock;

use minijinja::{Environment, UndefinedBehavior, context};
use regex::Regex;

// Compiling a regex is expensive: do it once, not per call or per request.
static TAG: LazyLock<Regex> =
    LazyLock::new(|| Regex::new(r"#([a-z][a-z0-9_-]{0,29})\b").expect("static pattern is valid"));
static DUE: LazyLock<Regex> = LazyLock::new(|| {
    Regex::new(r"(?P<year>\d{4})-(?P<month>0[1-9]|1[0-2])-(?P<day>0[1-9]|[12]\d|3[01])")
        .expect("static pattern is valid")
});

fn main() {
    let text =
        "Fix login #bug before #release-2 (due 2030-01-31). Also #Bug and #bug again, not C#.";

    let mut tags: Vec<&str> = TAG
        .captures_iter(text)
        .map(|c| c.get(1).unwrap().as_str())
        .collect();
    let mut seen = HashSet::new();
    tags.retain(|t| seen.insert(*t)); // unique, first-seen order
    println!("tags: {tags:?}");

    let caps = DUE.captures(text).unwrap();
    println!(
        "due: year={} month={} day={}",
        &caps["year"], &caps["month"], &caps["day"]
    );
    println!("redacted: {}", DUE.replace_all(text, "<date>"));
    println!(
        "'#Bug' (uppercase) matches the lowercase-only pattern: {}",
        TAG.is_match("#Bug")
    );

    // The `regex` crate guarantees linear time (no backtracking), so user-supplied input cannot cause ReDoS.
    // The price: no look-around or back-references. A bad pattern is an Err, not a panic:
    let user_pattern = "(unclosed";
    println!("invalid pattern: {}", Regex::new(user_pattern).is_err());

    // Templates: keep presentation out of handler code. Names ending in .html turn on auto-escaping.
    let mut env = Environment::new();
    env.add_template("reminder.txt", "Hi {{ name }}, {{ tasks | length }} task(s) due:\n{% for t in tasks %}- {{ t.title }} ({{ t.due }})\n{% endfor %}").unwrap();
    env.add_template("card.html", "<li class=\"task\">{{ title }}</li>")
        .unwrap();

    let out = env
        .get_template("reminder.txt")
        .unwrap()
        .render(context! {
            name => "Ada",
            tasks => vec![
                context! { title => "Write docs", due => "2030-01-31" },
                context! { title => "Ship <v1>", due => "2030-02-01" },
            ],
        })
        .unwrap();
    print!("{out}");

    let html = env
        .get_template("card.html")
        .unwrap()
        .render(context! { title => "<script>alert(1)</script>" })
        .unwrap();
    println!("{html}");

    // Strict mode turns a forgotten variable into an error instead of silently printing nothing.
    env.set_undefined_behavior(UndefinedBehavior::Strict);
    let missing = env
        .get_template("reminder.txt")
        .unwrap()
        .render(context! { name => "Ada" });
    println!(
        "strict mode rejects a missing variable: {}",
        missing.is_err()
    );
}
