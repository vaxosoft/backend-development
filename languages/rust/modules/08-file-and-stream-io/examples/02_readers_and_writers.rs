//! `Read` and `Write` are the abstraction: the same code works on files, memory and sockets.
use std::io::{self, BufRead, Cursor, Read, Write};

/// Wraps any writer and counts the bytes that actually went through.
struct CountingWriter<W> {
    inner: W,
    bytes: u64,
}

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

fn shout<R: BufRead, W: Write>(input: R, mut output: W) -> io::Result<()> {
    for line in input.lines() {
        writeln!(output, "{}!", line?.to_uppercase())?;
    }
    Ok(())
}

fn main() -> io::Result<()> {
    // A `&[u8]` is a Read, a `Vec<u8>` is a Write: no files needed to test I/O code.
    let mut sink = CountingWriter {
        inner: Vec::new(),
        bytes: 0,
    };
    shout(&b"write docs\nreview PR\n"[..], &mut sink)?;
    println!(
        "{} bytes: {:?}",
        sink.bytes,
        String::from_utf8_lossy(&sink.inner)
    );

    // `take` limits any reader: the building block for "max upload size".
    let mut limited = Cursor::new("0123456789").take(4);
    let mut head = String::new();
    limited.read_to_string(&mut head)?;
    println!("take(4): {head}");

    // `chain` concatenates readers; `io::copy` streams in small chunks.
    let mut chained = Cursor::new("header\n").chain(Cursor::new("body\n"));
    let mut copied = Vec::new();
    let n = io::copy(&mut chained, &mut copied)?;
    println!("copied {n} bytes: {:?}", String::from_utf8_lossy(&copied));

    // stdout is a Write too (locking it once avoids re-locking per line).
    let mut stdout = io::stdout().lock();
    shout(Cursor::new("done"), &mut stdout)?;

    // `read_exact` fails with UnexpectedEof instead of returning a short read.
    let mut four = [0u8; 4];
    let error = Cursor::new("ab").read_exact(&mut four).unwrap_err();
    println!("read_exact on 2 bytes -> {:?}", error.kind());
    Ok(())
}
