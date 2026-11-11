use m13_ex1_text_utils::{extract_tags, render_card_html, render_reminder, slugify};

#[test]
fn tags_are_lowercased_unique_and_in_order() {
    assert_eq!(
        extract_tags("Fix #Bug in #login, then #bug again and #release-2"),
        ["bug", "login", "release-2"]
    );
}

#[test]
fn hashes_inside_words_are_not_tags() {
    assert_eq!(
        extract_tags("Learn C# and F#, see a#b, issue ##double, #ok"),
        ["ok"]
    );
    assert!(extract_tags("no tags here, #1 is a number and # alone is nothing").is_empty());
}

#[test]
fn tags_can_sit_at_the_edges_and_next_to_punctuation() {
    assert_eq!(
        extract_tags("#start (#paren) end,#comma #end"),
        ["start", "paren", "comma", "end"]
    );
}

#[test]
fn tag_length_and_count_are_capped() {
    let long = format!("#{}", "a".repeat(40));
    assert_eq!(extract_tags(&long), ["a".repeat(30)]);

    let many: String = (1..=15).map(|i| format!("#t{i} ")).collect();
    let tags = extract_tags(&many);
    assert_eq!(tags.len(), 10);
    assert_eq!(tags.last().map(String::as_str), Some("t10"));
}

#[test]
fn slugify_collapses_everything_that_is_not_alphanumeric() {
    assert_eq!(slugify("Ship v2.0 -- now!"), "ship-v2-0-now");
    assert_eq!(slugify("  Hello,   World  "), "hello-world");
    assert_eq!(slugify("Déjà vu"), "d-j-vu");
    assert_eq!(slugify("***"), "");
    assert_eq!(slugify(""), "");
}

#[test]
fn reminder_lists_each_task() {
    let items = vec![
        ("Write docs".to_string(), "Thu 31 Jan 2030".to_string()),
        ("Ship it".to_string(), "Fri 01 Feb 2030".to_string()),
    ];
    assert_eq!(
        render_reminder("Ada", &items).unwrap(),
        "Hi Ada, you have 2 task(s) due:\n- Write docs (due Thu 31 Jan 2030)\n- Ship it (due Fri 01 Feb 2030)\n"
    );
}

#[test]
fn reminder_for_nothing_due_and_no_html_escaping_in_plain_text() {
    assert_eq!(
        render_reminder("Ada", &[]).unwrap(),
        "Hi Ada, nothing is due.\n"
    );

    let items = vec![(
        "Fix <b>bold</b> & \"quotes\"".to_string(),
        "today".to_string(),
    )];
    let text = render_reminder("Ada", &items).unwrap();
    assert!(
        text.contains("- Fix <b>bold</b> & \"quotes\" (due today)"),
        "got: {text}"
    );
}

#[test]
fn html_cards_escape_the_title() {
    let html = render_card_html("<script>alert(1)</script>").unwrap();
    assert!(
        html.starts_with("<li class=\"task\">") && html.ends_with("</li>"),
        "got: {html}"
    );
    assert!(!html.contains("<script>"), "got: {html}");
    assert!(html.contains("&lt;script&gt;"), "got: {html}");
    assert_eq!(
        render_card_html("plain").unwrap(),
        "<li class=\"task\">plain</li>"
    );
}
