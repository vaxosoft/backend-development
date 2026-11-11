//! Parsing of the `ids` query parameter.

use crate::BadRequest;

/// Largest number of ids accepted in one request.
pub const MAX_IDS: usize = 50;

/// Turns `"1,2,3"` into `[1, 2, 3]`.
pub fn parse_ids(raw: Option<&str>) -> Result<Vec<u64>, BadRequest> {
    let raw = raw.map(str::trim).filter(|s| !s.is_empty());
    let Some(raw) = raw else {
        return Err(BadRequest("query parameter `ids` is required".into()));
    };
    let ids = raw
        .split(',')
        .map(|part| {
            part.trim()
                .parse::<u64>()
                .map_err(|_| BadRequest(format!("`{}` is not a valid id", part.trim())))
        })
        .collect::<Result<Vec<_>, _>>()?;
    if ids.len() > MAX_IDS {
        return Err(BadRequest(format!("at most {MAX_IDS} ids are allowed")));
    }
    Ok(ids)
}
