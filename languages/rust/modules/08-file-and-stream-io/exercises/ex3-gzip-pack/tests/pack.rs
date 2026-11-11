use std::fs;
use std::io::{self, Cursor, Read};
use std::path::Path;

use m08_ex3_gzip_pack::{PackError, pack, unpack_verified};
use sha2::{Digest, Sha256};

fn sha256_hex(data: &[u8]) -> String {
    hex::encode(Sha256::digest(data))
}

fn sample(len: usize) -> Vec<u8> {
    (0..len).map(|i| ((i / 7) % 251) as u8).collect()
}

fn names(dir: &Path) -> Vec<String> {
    let mut names: Vec<String> = fs::read_dir(dir)
        .unwrap()
        .map(|e| e.unwrap().file_name().to_string_lossy().into_owned())
        .collect();
    names.sort();
    names
}

#[test]
fn pack_then_unpack_restores_the_original() {
    let dir = tempfile::tempdir().unwrap();
    let data = sample(300_000);
    let packed = dir.path().join("data.gz");
    let restored = dir.path().join("data.out");

    let info = pack(Cursor::new(&data), &packed).unwrap();
    assert_eq!(info.raw_len, 300_000);
    assert_eq!(info.sha256, sha256_hex(&data));
    assert_eq!(info.packed_len, fs::metadata(&packed).unwrap().len());
    assert!(
        info.packed_len < info.raw_len / 2,
        "repetitive data must compress"
    );

    let len = unpack_verified(&packed, &restored, &info.sha256).unwrap();
    assert_eq!(len, 300_000);
    assert_eq!(fs::read(&restored).unwrap(), data);
    assert_eq!(
        names(dir.path()),
        ["data.gz", "data.out"],
        "no temp files left behind"
    );
}

#[test]
fn the_packed_file_is_real_gzip() {
    let dir = tempfile::tempdir().unwrap();
    let packed = dir.path().join("a.gz");
    pack(Cursor::new("hello gzip"), &packed).unwrap();

    let mut text = String::new();
    flate2::read::GzDecoder::new(fs::File::open(&packed).unwrap())
        .read_to_string(&mut text)
        .unwrap();
    assert_eq!(text, "hello gzip");
}

#[test]
fn empty_input_round_trips() {
    let dir = tempfile::tempdir().unwrap();
    let packed = dir.path().join("empty.gz");
    let out = dir.path().join("empty.out");

    let info = pack(io::empty(), &packed).unwrap();
    assert_eq!(
        (info.raw_len, info.sha256.as_str()),
        (
            0,
            "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
        )
    );
    assert_eq!(unpack_verified(&packed, &out, &info.sha256).unwrap(), 0);
}

struct FailsAfter(usize);

impl Read for FailsAfter {
    fn read(&mut self, buf: &mut [u8]) -> io::Result<usize> {
        if self.0 == 0 {
            return Err(io::Error::other("disk on fire"));
        }
        let n = buf.len().min(self.0).min(16);
        buf[..n].fill(b'q');
        self.0 -= n;
        Ok(n)
    }
}

#[test]
fn a_failing_source_keeps_the_previous_file_and_leaves_no_temp_file() {
    let dir = tempfile::tempdir().unwrap();
    let dest = dir.path().join("keep.gz");
    fs::write(&dest, b"previous content").unwrap();

    let result = pack(FailsAfter(100), &dest);
    assert!(matches!(result, Err(PackError::Io(_))), "got {result:?}");
    assert_eq!(fs::read(&dest).unwrap(), b"previous content");
    assert_eq!(names(dir.path()), ["keep.gz"]);
}

#[test]
fn a_checksum_mismatch_creates_nothing() {
    let dir = tempfile::tempdir().unwrap();
    let packed = dir.path().join("a.gz");
    let out = dir.path().join("a.out");
    pack(Cursor::new("some data"), &packed).unwrap();

    let wrong = sha256_hex(b"other data");
    let result = unpack_verified(&packed, &out, &wrong);
    match result {
        Err(PackError::ChecksumMismatch { expected, actual }) => {
            assert_eq!(expected, wrong);
            assert_eq!(actual, sha256_hex(b"some data"));
        }
        other => panic!("expected ChecksumMismatch, got {other:?}"),
    }
    assert_eq!(names(dir.path()), ["a.gz"]);
}

#[test]
fn a_checksum_mismatch_does_not_overwrite_an_existing_file() {
    let dir = tempfile::tempdir().unwrap();
    let packed = dir.path().join("a.gz");
    let out = dir.path().join("a.out");
    pack(Cursor::new("new"), &packed).unwrap();
    fs::write(&out, "old").unwrap();

    assert!(unpack_verified(&packed, &out, &sha256_hex(b"nope")).is_err());
    assert_eq!(fs::read_to_string(&out).unwrap(), "old");
}

#[test]
fn a_truncated_archive_is_an_io_error() {
    let dir = tempfile::tempdir().unwrap();
    let packed = dir.path().join("a.gz");
    let out = dir.path().join("a.out");
    let data = sample(100_000);
    let info = pack(Cursor::new(&data), &packed).unwrap();

    let bytes = fs::read(&packed).unwrap();
    fs::write(&packed, &bytes[..bytes.len() / 2]).unwrap();

    let result = unpack_verified(&packed, &out, &info.sha256);
    assert!(matches!(result, Err(PackError::Io(_))), "got {result:?}");
    assert_eq!(names(dir.path()), ["a.gz"]);
}

#[test]
fn a_missing_archive_is_an_io_error() {
    let dir = tempfile::tempdir().unwrap();
    let result = unpack_verified(&dir.path().join("nope.gz"), &dir.path().join("out"), "00");
    assert!(matches!(result, Err(PackError::Io(e)) if e.kind() == io::ErrorKind::NotFound));
}
