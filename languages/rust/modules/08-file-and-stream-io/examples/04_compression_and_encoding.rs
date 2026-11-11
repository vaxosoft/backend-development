//! Streaming gzip, base64 and hex.
use std::io::{Cursor, Read, Write};

use base64::{
    Engine,
    engine::general_purpose::{STANDARD, URL_SAFE_NO_PAD},
};
use flate2::{Compression, read::GzDecoder, write::GzEncoder};

fn main() -> anyhow::Result<()> {
    let text = "task tracker ".repeat(1000);

    // Encoder wraps a writer; `finish` writes the gzip trailer and returns the inner writer.
    let mut encoder = GzEncoder::new(Vec::new(), Compression::default());
    encoder.write_all(text.as_bytes())?;
    let compressed = encoder.finish()?;
    println!("gzip magic: {:02x}{:02x}", compressed[0], compressed[1]);
    println!("smaller than the input: {}", compressed.len() < text.len());

    // Decoder wraps a reader, so it composes with `read_to_string`, `io::copy`, `lines()`...
    let mut restored = String::new();
    GzDecoder::new(Cursor::new(&compressed)).read_to_string(&mut restored)?;
    println!("round trip equal: {}", restored == text);

    // Garbage is an error, not a panic.
    let error = GzDecoder::new(&b"definitely not gzip data"[..])
        .read_to_end(&mut Vec::new())
        .unwrap_err();
    println!("invalid gzip -> {:?}", error.kind());

    let data = b"Hello, Task Tracker?!";
    println!("hex:        {}", hex::encode(data));
    println!("base64:     {}", STANDARD.encode(data));
    // Bytes that map to `+` and `/` in the standard alphabet show the difference.
    let binary = [0xfb, 0xff, 0xfe];
    println!("standard:   {}", STANDARD.encode(binary));
    println!("url safe:   {}", URL_SAFE_NO_PAD.encode(binary));
    let decoded = STANDARD.decode("SGVsbG8=")?;
    println!("decoded: {}", String::from_utf8(decoded)?);
    println!("bad base64 -> {}", STANDARD.decode("***").unwrap_err());
    println!("bad hex    -> {}", hex::decode("xyz").unwrap_err());
    Ok(())
}
