//! A minimal `GET /healthz` client, so a container without `curl` can still
//! be health-checked: `HEALTHCHECK CMD ["tasks-service", "healthcheck"]`.

use std::io::{Read, Write};
use std::net::{TcpStream, ToSocketAddrs};
use std::time::Duration;

const TIMEOUT: Duration = Duration::from_secs(2);

/// `true` if `GET /healthz` on `addr` (`host:port`) answers `200`.
pub fn probe(addr: &str) -> bool {
    let Some(target) = addr.to_socket_addrs().ok().and_then(|mut a| a.next()) else {
        return false;
    };
    let Ok(mut stream) = TcpStream::connect_timeout(&target, TIMEOUT) else {
        return false;
    };
    let _ = stream.set_read_timeout(Some(TIMEOUT));
    if stream
        .write_all(b"GET /healthz HTTP/1.0\r\nHost: localhost\r\n\r\n")
        .is_err()
    {
        return false;
    }
    let mut status_line = [0u8; 16];
    stream.read(&mut status_line).is_ok_and(|n| {
        let head = String::from_utf8_lossy(&status_line[..n]);
        head.starts_with("HTTP/1.") && head.get(8..12) == Some(" 200")
    })
}
