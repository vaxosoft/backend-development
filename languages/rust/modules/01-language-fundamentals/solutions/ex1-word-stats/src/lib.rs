//! Exercise 1: borrowing, `&str` slices and iterators.

use std::collections::BTreeMap;

/// Splits `text` on whitespace, trims leading/trailing punctuation from every
/// word (keep inner characters such as the apostrophe in "don't"), and drops
/// words that become empty.
fn words(text: &str) -> impl Iterator<Item = &str> {
    text.split_whitespace()
        .map(|w| w.trim_matches(|c: char| !c.is_alphanumeric()))
        .filter(|w| !w.is_empty())
}

/// Counts words case-insensitively.
pub fn word_counts(text: &str) -> BTreeMap<String, usize> {
    let mut counts = BTreeMap::new();
    for word in words(text) {
        *counts.entry(word.to_lowercase()).or_insert(0) += 1;
    }
    counts
}

/// Returns the longest word (by `char` count) as a slice of `text`; the first
/// one wins on ties. No allocation: the result borrows from the input.
pub fn longest_word(text: &str) -> Option<&str> {
    // `max_by_key` returns the LAST maximum, so fold to keep the first.
    words(text).fold(None, |best: Option<&str>, word| match best {
        Some(b) if b.chars().count() >= word.chars().count() => Some(b),
        _ => Some(word),
    })
}

/// The `n` most frequent words, ordered by count (descending) then word
/// (ascending). The returned `&str`s borrow from `counts`.
pub fn top_words(counts: &BTreeMap<String, usize>, n: usize) -> Vec<(&str, usize)> {
    let mut entries: Vec<(&str, usize)> = counts.iter().map(|(w, c)| (w.as_str(), *c)).collect();
    entries.sort_by(|a, b| b.1.cmp(&a.1).then(a.0.cmp(b.0)));
    entries.truncate(n);
    entries
}
