use std::io::{self, Cursor, Read};
use std::path::Path;

use m08_ex2_safe_upload_store::{StoreError, sanitize_filename, store_upload};

const HELLO_SHA256: &str = "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824";

fn names(dir: &Path) -> Vec<String> {
    let mut names: Vec<String> = std::fs::read_dir(dir)
        .unwrap()
        .map(|e| e.unwrap().file_name().to_string_lossy().into_owned())
        .collect();
    names.sort();
    names
}

#[test]
fn sanitize_keeps_only_the_last_component() {
    let cases = [
        ("report.pdf", Some("report.pdf")),
        ("../../etc/passwd", Some("passwd")),
        ("/etc/shadow", Some("shadow")),
        ("C:\\Users\\bob\\a.txt", Some("a.txt")),
        ("  spaced name.txt  ", Some("spaced name.txt")),
        ("evil\0name\n.txt", Some("evilname.txt")),
        ("dir/", None),
        ("..", None),
        (".", None),
        ("", None),
        ("   ", None),
        ("a/..", None),
    ];
    for (input, expected) in cases {
        assert_eq!(
            sanitize_filename(input).as_deref(),
            expected,
            "input {input:?}"
        );
    }
}

#[test]
fn stores_content_under_the_id_and_reports_size_and_hash() {
    let root = tempfile::tempdir().unwrap();
    let stored = store_upload(
        root.path(),
        "f1",
        "greeting.txt",
        Cursor::new("hello"),
        1024,
    )
    .unwrap();

    assert_eq!(stored.name, "greeting.txt");
    assert_eq!(stored.path, root.path().join("f1"));
    assert_eq!((stored.size, stored.sha256.as_str()), (5, HELLO_SHA256));
    assert_eq!(std::fs::read_to_string(&stored.path).unwrap(), "hello");
    assert_eq!(names(root.path()), ["f1"], "no temp files left behind");
}

#[test]
fn a_hostile_client_name_never_influences_the_path() {
    let root = tempfile::tempdir().unwrap();
    let stored = store_upload(root.path(), "f2", "../../etc/passwd", Cursor::new("x"), 10).unwrap();

    assert_eq!(stored.name, "passwd");
    assert_eq!(stored.path.parent().unwrap(), root.path());
    assert_eq!(names(root.path()), ["f2"]);
}

#[test]
fn an_unusable_client_name_falls_back_to_a_default() {
    let root = tempfile::tempdir().unwrap();
    let stored = store_upload(root.path(), "f3", "..", Cursor::new("x"), 10).unwrap();
    assert_eq!(stored.name, "upload");
}

#[test]
fn ids_that_could_escape_the_root_are_rejected() {
    let root = tempfile::tempdir().unwrap();
    for id in ["", "..", "../evil", "a/b", "a\\b", "with space", "nul\0"] {
        let result = store_upload(root.path(), id, "x.txt", Cursor::new("x"), 10);
        assert!(matches!(result, Err(StoreError::InvalidId)), "id {id:?}");
    }
    assert!(names(root.path()).is_empty());
}

#[test]
fn a_body_of_exactly_the_limit_is_accepted() {
    let root = tempfile::tempdir().unwrap();
    let stored = store_upload(
        root.path(),
        "exact",
        "x",
        io::repeat(b'a').take(20_000),
        20_000,
    )
    .unwrap();
    assert_eq!(stored.size, 20_000);
}

#[test]
fn one_byte_over_the_limit_is_rejected_and_leaves_nothing_behind() {
    let root = tempfile::tempdir().unwrap();
    let result = store_upload(
        root.path(),
        "big",
        "x",
        io::repeat(b'a').take(20_001),
        20_000,
    );

    assert!(
        matches!(result, Err(StoreError::TooLarge { limit: 20_000 })),
        "got {result:?}"
    );
    assert!(
        names(root.path()).is_empty(),
        "partial file must be removed"
    );
}

#[test]
fn an_endless_body_is_cut_off_instead_of_filling_the_disk() {
    let root = tempfile::tempdir().unwrap();
    let result = store_upload(root.path(), "endless", "x", io::repeat(b'z'), 100_000);
    assert!(matches!(result, Err(StoreError::TooLarge { .. })));
    assert!(names(root.path()).is_empty());
}

struct FailsAfter(usize);

impl Read for FailsAfter {
    fn read(&mut self, buf: &mut [u8]) -> io::Result<usize> {
        if self.0 == 0 {
            return Err(io::Error::new(
                io::ErrorKind::ConnectionReset,
                "client went away",
            ));
        }
        let n = buf.len().min(self.0).min(4);
        buf[..n].fill(b'x');
        self.0 -= n;
        Ok(n)
    }
}

#[test]
fn a_read_error_midway_leaves_nothing_behind() {
    let root = tempfile::tempdir().unwrap();
    let result = store_upload(root.path(), "broken", "x", FailsAfter(10), 1000);

    assert!(
        matches!(&result, Err(StoreError::Io(e)) if e.kind() == io::ErrorKind::ConnectionReset),
        "got {result:?}"
    );
    assert!(names(root.path()).is_empty());
}
