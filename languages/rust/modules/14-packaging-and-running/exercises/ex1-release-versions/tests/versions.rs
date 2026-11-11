use m14_ex1_release_versions::{Part, Version, VersionError};

fn v(text: &str) -> Version {
    Version::parse(text).unwrap_or_else(|e| panic!("{text}: {e}"))
}

#[test]
fn parses_plain_prefixed_and_prerelease_versions() {
    assert_eq!(
        v("1.4.2"),
        Version {
            major: 1,
            minor: 4,
            patch: 2,
            pre: None
        }
    );
    assert_eq!(
        v("v10.0.3"),
        Version {
            major: 10,
            minor: 0,
            patch: 3,
            pre: None
        }
    );
    assert_eq!(v("1.5.0-rc.1").pre.as_deref(), Some("rc.1"));
    assert_eq!(
        v("0.1.0-alpha-2").pre.as_deref(),
        Some("alpha-2"),
        "only the first '-' splits"
    );
}

#[test]
fn rejects_malformed_versions() {
    for bad in [
        "",
        "1",
        "1.2",
        "1.2.3.4",
        "a.b.c",
        "1.2.x",
        "01.2.3",
        "1.02.3",
        "1.2.-3",
        "1.0.0-",
        "1.0.0-rc_1",
        "1.0.0+build5",
        "V1.2.3",
        " 1.2.3",
        "1.2.3 ",
        "99999999999999999999.0.0",
    ] {
        assert_eq!(
            Version::parse(bad),
            Err(VersionError::Invalid(bad.to_string())),
            "input {bad:?}"
        );
    }
}

#[test]
fn display_round_trips() {
    for text in ["0.0.0", "1.4.2", "2.0.0-beta.3"] {
        assert_eq!(v(text).to_string(), text);
    }
    assert_eq!(
        v("v1.4.2").to_string(),
        "1.4.2",
        "the v prefix is not part of the version"
    );
}

#[test]
fn bumping_resets_lower_parts_and_drops_the_prerelease() {
    assert_eq!(v("1.4.2").bump(Part::Patch).to_string(), "1.4.3");
    assert_eq!(v("1.4.2").bump(Part::Minor).to_string(), "1.5.0");
    assert_eq!(v("1.4.2").bump(Part::Major).to_string(), "2.0.0");
    assert_eq!(v("1.4.2-rc.1").bump(Part::Patch).to_string(), "1.4.3");
}

#[test]
fn stable_releases_move_the_floating_tags() {
    assert_eq!(v("1.4.2").image_tags(false), ["1.4.2", "1.4", "1"]);
    assert_eq!(v("1.4.2").image_tags(true), ["1.4.2", "1.4", "1", "latest"]);
    assert_eq!(v("0.9.0").image_tags(true), ["0.9.0", "0.9", "0", "latest"]);
}

#[test]
fn prereleases_publish_only_their_exact_tag() {
    assert_eq!(v("1.5.0-rc.1").image_tags(true), ["1.5.0-rc.1"]);
    assert_eq!(v("1.5.0-rc.1").image_tags(false), ["1.5.0-rc.1"]);
}
