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
    let _ = words(text);
    todo!("count lowercase words into a BTreeMap using the entry API")
}

/// Returns the longest word (by `char` count) as a slice of `text`; the first
/// one wins on ties. No allocation: the result borrows from the input.
pub fn longest_word(text: &str) -> Option<&str> {
    let _ = words(text);
    todo!("pick the longest word without cloning")
}

/// The `n` most frequent words, ordered by count (descending) then word
/// (ascending). The returned `&str`s borrow from `counts`.
pub fn top_words(counts: &BTreeMap<String, usize>, n: usize) -> Vec<(&str, usize)> {
    let _ = (counts, n);
    todo!("collect, sort and truncate")
}
