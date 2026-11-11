use m03_ex2_validate_user::{
    User, UserInput, UserPatch, UserPatchInput, apply_patch, validate_user, validate_user_patch,
};

fn input(name: Option<&str>, email: Option<&str>) -> UserInput {
    UserInput {
        name: name.map(String::from),
        email: email.map(String::from),
    }
}

fn patch_from_json(json: &str) -> UserPatchInput {
    serde_json::from_str(json).unwrap()
}

fn problems(
    result: Result<impl std::fmt::Debug, m03_ex2_validate_user::FieldErrors>,
) -> Vec<(String, String)> {
    result.unwrap_err().into_iter().collect()
}

fn pair(field: &str, message: &str) -> (String, String) {
    (field.to_string(), message.to_string())
}

#[test]
fn a_valid_user_is_trimmed() {
    let user = validate_user(input(Some("  Ada Lovelace "), Some(" ada@example.com\n"))).unwrap();
    assert_eq!(
        user,
        User {
            name: "Ada Lovelace".into(),
            email: "ada@example.com".into()
        }
    );
}

#[test]
fn missing_fields_are_reported_per_field() {
    assert_eq!(
        problems(validate_user(input(None, None))),
        [pair("email", "is required"), pair("name", "is required")]
    );
}

#[test]
fn every_violation_is_reported_at_once() {
    assert_eq!(
        problems(validate_user(input(Some("   "), Some("not-an-email")))),
        [
            pair("email", "must be an email address"),
            pair("name", "must be 1-100 characters")
        ]
    );
}

#[test]
fn name_length_is_counted_in_characters() {
    let hundred = "é".repeat(100);
    assert!(validate_user(input(Some(&hundred), Some("a@b.c"))).is_ok());
    let too_long = "é".repeat(101);
    assert!(validate_user(input(Some(&too_long), Some("a@b.c"))).is_err());
}

#[test]
fn emails_need_one_at_sign_with_both_sides() {
    for bad in ["@example.com", "ada@", "a@b@c", "plain", ""] {
        assert!(
            validate_user(input(Some("Ada"), Some(bad))).is_err(),
            "{bad:?} should be rejected"
        );
    }
}

#[test]
fn json_distinguishes_absent_from_null() {
    let absent = patch_from_json("{}");
    assert_eq!(absent.name, None);
    let null = patch_from_json(r#"{"name":null}"#);
    assert_eq!(null.name, Some(None));
    let value = patch_from_json(r#"{"name":"Ada"}"#);
    assert_eq!(value.name, Some(Some("Ada".to_string())));
}

#[test]
fn patch_validates_only_present_fields() {
    let patch = validate_user_patch(patch_from_json(r#"{"email":" new@example.com "}"#)).unwrap();
    assert_eq!(
        patch,
        UserPatch {
            name: None,
            email: Some("new@example.com".into())
        }
    );
}

#[test]
fn patch_rejects_null_and_invalid_values() {
    assert_eq!(
        problems(validate_user_patch(patch_from_json(
            r#"{"name":null,"email":"nope"}"#
        ))),
        [
            pair("email", "must be an email address"),
            pair("name", "must not be null")
        ]
    );
}

#[test]
fn an_empty_patch_is_an_error() {
    assert_eq!(
        problems(validate_user_patch(patch_from_json("{}"))),
        [pair("body", "at least one field is required")]
    );
}

#[test]
fn applying_a_patch_keeps_untouched_fields() {
    let user = User {
        name: "Ada".into(),
        email: "ada@example.com".into(),
    };
    let patch = UserPatch {
        name: Some("Ada L.".into()),
        email: None,
    };
    assert_eq!(
        apply_patch(&user, &patch),
        User {
            name: "Ada L.".into(),
            email: "ada@example.com".into()
        }
    );
}
