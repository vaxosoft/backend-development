use std::io::{self, BufWriter, Write};

use m08_ex1_counting_writer::{CountingWriter, write_lines};

/// Accepts at most `chunk` bytes per call, like a socket with a small send buffer.
struct Trickle {
    data: Vec<u8>,
    chunk: usize,
    flushes: usize,
}

impl Write for Trickle {
    fn write(&mut self, buf: &[u8]) -> io::Result<usize> {
        let n = buf.len().min(self.chunk);
        self.data.extend_from_slice(&buf[..n]);
        Ok(n)
    }

    fn flush(&mut self) -> io::Result<()> {
        self.flushes += 1;
        Ok(())
    }
}

struct Broken;

impl Write for Broken {
    fn write(&mut self, _buf: &[u8]) -> io::Result<usize> {
        Err(io::Error::new(io::ErrorKind::BrokenPipe, "closed"))
    }

    fn flush(&mut self) -> io::Result<()> {
        Ok(())
    }
}

#[test]
fn counts_bytes_written_to_a_vec() {
    let mut writer = CountingWriter::new(Vec::new());
    writer.write_all(b"hello ").unwrap();
    writer.write_all(b"world").unwrap();
    assert_eq!(writer.bytes_written(), 11);
    assert_eq!(writer.into_inner(), b"hello world");
}

#[test]
fn counts_what_was_accepted_not_what_was_offered() {
    let mut writer = CountingWriter::new(Trickle {
        data: Vec::new(),
        chunk: 3,
        flushes: 0,
    });
    assert_eq!(writer.write(b"abcdefgh").unwrap(), 3);
    assert_eq!(writer.bytes_written(), 3);

    writer.write_all(b"0123456789").unwrap();
    assert_eq!(writer.bytes_written(), 13);
    assert_eq!(writer.into_inner().data, b"abc0123456789");
}

#[test]
fn errors_are_propagated_and_not_counted() {
    let mut writer = CountingWriter::new(Broken);
    let error = writer.write(b"data").unwrap_err();
    assert_eq!(error.kind(), io::ErrorKind::BrokenPipe);
    assert_eq!(writer.bytes_written(), 0);
}

#[test]
fn flush_reaches_the_inner_writer() {
    let mut writer = CountingWriter::new(Trickle {
        data: Vec::new(),
        chunk: 8,
        flushes: 0,
    });
    writer.flush().unwrap();
    writer.flush().unwrap();
    assert_eq!(writer.into_inner().flushes, 2);
}

#[test]
fn composes_with_bufwriter() {
    let counter = CountingWriter::new(Vec::new());
    let mut buffered = BufWriter::new(counter);
    buffered.write_all(b"buffered").unwrap();
    // Nothing reached the counter yet: BufWriter holds it back until flushed.
    assert_eq!(buffered.get_ref().bytes_written(), 0);
    buffered.flush().unwrap();
    assert_eq!(buffered.get_ref().bytes_written(), 8);
}

#[test]
fn write_lines_reports_the_total_including_newlines() {
    let mut out = Vec::new();
    let total = write_lines(&mut out, &["write docs", "ship it"]).unwrap();
    assert_eq!(out, b"write docs\nship it\n");
    assert_eq!(total, 19);
}

#[test]
fn write_lines_works_with_short_writes() {
    let mut trickle = Trickle {
        data: Vec::new(),
        chunk: 2,
        flushes: 0,
    };
    assert_eq!(write_lines(&mut trickle, &["abc", "de"]).unwrap(), 7);
    assert_eq!(trickle.data, b"abc\nde\n");
    assert_eq!(trickle.flushes, 1);
}
