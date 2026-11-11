//! Parsing of the `ids` query parameter.

use crate::BadRequest;

/// Largest number of ids accepted in one request.
pub const MAX_IDS: usize = 50;

/// Turns `"1,2,3"` into `[1, 2, 3]`.
///
/// TODO: reject (with a helpful message) a missing or empty value, more than
/// `MAX_IDS` entries, and any entry that is not an unsigned integer
/// (`1,,2`, `abc`, `-1`). Spaces around an entry are allowed. Keep the order.
pub fn parse_ids(_raw: Option<&str>) -> Result<Vec<u64>, BadRequest> {
    todo!("parse and validate the ids query parameter")
}
