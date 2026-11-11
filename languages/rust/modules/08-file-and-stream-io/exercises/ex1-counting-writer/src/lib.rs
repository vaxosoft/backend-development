//! Exercise 1: a `Write` adapter that counts the bytes that were really written.
use std::io::{self, Write};

// The fields are used by your implementation; the starter does not read them yet.
#[allow(dead_code)]
pub struct CountingWriter<W> {
    inner: W,
    bytes: u64,
}

impl<W: Write> CountingWriter<W> {
    pub fn new(inner: W) -> Self {
        Self { inner, bytes: 0 }
    }

    /// Total bytes the inner writer has accepted so far.
    pub fn bytes_written(&self) -> u64 {
        todo!("return the counter")
    }

    pub fn into_inner(self) -> W {
        todo!("give the inner writer back")
    }
}

/// Forward to the inner writer and count what it accepted.
///
/// `write` may accept FEWER bytes than offered, and may fail: count the number it returns,
/// never `buf.len()`, and count nothing on error. `flush` must reach the inner writer.
impl<W: Write> Write for CountingWriter<W> {
    fn write(&mut self, _buf: &[u8]) -> io::Result<usize> {
        todo!("delegate and count")
    }

    fn flush(&mut self) -> io::Result<()> {
        todo!("delegate")
    }
}

/// Write one line per entry (`"{line}\n"`) to `out` and return how many bytes were written.
pub fn write_lines<W: Write>(out: W, lines: &[&str]) -> io::Result<u64> {
    let mut counter = CountingWriter::new(out);
    for line in lines {
        writeln!(counter, "{line}")?;
    }
    counter.flush()?;
    Ok(counter.bytes_written())
}
