use m01_ex1_word_stats::{longest_word, top_words, word_counts};

#[test]
fn counts_ignore_case_and_punctuation() {
    let counts = word_counts("The cat. THE dog, the bird!");
    assert_eq!(counts.get("the"), Some(&3));
    assert_eq!(counts.get("cat"), Some(&1));
    assert_eq!(counts.len(), 4);
}

#[test]
fn counts_keep_inner_apostrophes_and_skip_bare_punctuation() {
    let counts = word_counts("don't - stop -- don't");
    assert_eq!(counts.get("don't"), Some(&2));
    assert_eq!(counts.get("stop"), Some(&1));
    assert_eq!(counts.len(), 2);
}

#[test]
fn empty_text_has_no_words() {
    assert!(word_counts("  \n ").is_empty());
    assert_eq!(longest_word(""), None);
}

#[test]
fn longest_word_borrows_from_input_and_first_wins_ties() {
    let text = String::from("a quick brown fox");
    let longest = longest_word(&text).unwrap();
    assert_eq!(longest, "quick");
    // The result is a sub-slice of `text`, not a copy.
    let start = longest.as_ptr() as usize - text.as_ptr() as usize;
    assert_eq!(start, 2);
}

#[test]
fn longest_word_counts_chars_not_bytes() {
    assert_eq!(longest_word("日本語 abcd"), Some("abcd"));
}

#[test]
fn top_words_orders_by_count_then_alphabetically() {
    let counts = word_counts("b a c b a d b");
    assert_eq!(top_words(&counts, 3), vec![("b", 3), ("a", 2), ("c", 1)]);
    assert_eq!(top_words(&counts, 10).len(), 4);
    assert!(top_words(&counts, 0).is_empty());
}
