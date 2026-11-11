use std::net::SocketAddr;
use std::time::Duration;

use m02_ex3_graceful_shutdown::{Hooks, Outcome, router, serve};
use tokio::io::{AsyncReadExt, AsyncWriteExt};
use tokio::net::{TcpListener, TcpStream};
use tokio::sync::oneshot;
use tokio::task::JoinHandle;

struct Running {
    addr: SocketAddr,
    hooks: Hooks,
    stop: oneshot::Sender<()>,
    server: JoinHandle<std::io::Result<Outcome>>,
}

async fn start(grace: Duration) -> Running {
    let listener = TcpListener::bind("127.0.0.1:0").await.unwrap();
    let addr = listener.local_addr().unwrap();
    let hooks = Hooks::default();
    let (stop, stopped) = oneshot::channel::<()>();
    let server = tokio::spawn(serve(
        listener,
        router(hooks.clone()),
        async move {
            let _ = stopped.await;
        },
        grace,
    ));
    Running {
        addr,
        hooks,
        stop,
        server,
    }
}

async fn get(addr: SocketAddr, path: &str) -> std::io::Result<String> {
    let mut stream = TcpStream::connect(addr).await?;
    let request = format!("GET {path} HTTP/1.1\r\nHost: test\r\nConnection: close\r\n\r\n");
    stream.write_all(request.as_bytes()).await?;
    let mut response = String::new();
    stream.read_to_string(&mut response).await?;
    Ok(response)
}

async fn handler_entered(hooks: &Hooks) {
    tokio::time::timeout(Duration::from_secs(2), hooks.entered.notified())
        .await
        .expect("the /wait handler never started");
}

async fn wait_until_refused(addr: SocketAddr) {
    for _ in 0..200 {
        if TcpStream::connect(addr).await.is_err() {
            return;
        }
        tokio::time::sleep(Duration::from_millis(5)).await;
    }
    panic!("the listener still accepts connections after shutdown began");
}

#[tokio::test]
async fn serves_requests_until_shutdown() {
    let running = start(Duration::from_secs(5)).await;
    let response = get(running.addr, "/ping").await.unwrap();
    assert!(response.starts_with("HTTP/1.1 200"), "{response}");
    assert!(response.ends_with("pong"), "{response}");

    running.stop.send(()).unwrap();
    assert_eq!(running.server.await.unwrap().unwrap(), Outcome::Clean);
}

#[tokio::test]
async fn in_flight_requests_finish_and_new_connections_are_refused() {
    let running = start(Duration::from_secs(5)).await;
    let in_flight = tokio::spawn(get(running.addr, "/wait"));
    handler_entered(&running.hooks).await;

    running.stop.send(()).unwrap();
    wait_until_refused(running.addr).await;
    assert!(
        !in_flight.is_finished(),
        "the request must still be in flight"
    );

    running.hooks.release.notify_one();
    let response = in_flight.await.unwrap().unwrap();
    assert!(response.starts_with("HTTP/1.1 200"), "{response}");
    assert!(response.ends_with("released"), "{response}");
    assert_eq!(running.server.await.unwrap().unwrap(), Outcome::Clean);
}

#[tokio::test]
async fn a_stuck_request_cannot_block_shutdown_past_the_grace_period() {
    let running = start(Duration::from_millis(50)).await;
    let _stuck = tokio::spawn(get(running.addr, "/wait"));
    handler_entered(&running.hooks).await;

    running.stop.send(()).unwrap();
    let outcome = tokio::time::timeout(Duration::from_secs(3), running.server)
        .await
        .expect("serve must return once the grace period is over")
        .unwrap()
        .unwrap();
    assert_eq!(outcome, Outcome::TimedOut);
}
