//! Exercise 1: a `Write` adapter that counts the bytes that were really written.
use std::io::{self, Write};

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
        self.bytes
    }

    pub fn into_inner(self) -> W {
        self.inner
    }
}

/// Forward to the inner writer and count what it accepted.
///
/// `write` may accept FEWER bytes than offered, and may fail: count the number it returns,
/// never `buf.len()`, and count nothing on error. `flush` must reach the inner writer.
impl<W: Write> Write for CountingWriter<W> {
    fn write(&mut self, buf: &[u8]) -> io::Result<usize> {
        let written = self.inner.write(buf)?;
        self.bytes += written as u64;
        Ok(written)
    }

    fn flush(&mut self) -> io::Result<()> {
        self.inner.flush()
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
