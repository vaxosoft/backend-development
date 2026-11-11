//! Black-box tests: a real server on an ephemeral port, a fresh temp
//! directory per test, hand-built multipart bodies over HTTP.

use std::path::{Path, PathBuf};

use p3_file_service::{Config, build_app};
use reqwest::{Client, Response, StatusCode};
use serde_json::Value;
use sha2::{Digest, Sha256};
use tempfile::TempDir;
use tokio::net::TcpListener;

const BOUNDARY: &str = "----p3-test-boundary";
const MIB: usize = 1024 * 1024;

struct Server {
    base: String,
    client: Client,
    /// Holds the temp dir alive; the storage dir is `root/storage`.
    root: TempDir,
}

impl Server {
    async fn start() -> Self {
        Self::start_with_limit(Config::default().max_size).await
    }

    async fn start_with_limit(max_size: u64) -> Self {
        let root = TempDir::new().unwrap();
        let config = Config {
            storage_dir: root.path().join("storage"),
            max_size,
        };
        let app = build_app(&config).unwrap();
        let listener = TcpListener::bind("127.0.0.1:0").await.unwrap();
        let base = format!("http://{}", listener.local_addr().unwrap());
        tokio::spawn(async move { axum::serve(listener, app).await.unwrap() });
        Self {
            base,
            client: Client::new(),
            root,
        }
    }

    fn storage(&self) -> PathBuf {
        self.root.path().join("storage")
    }

    fn storage_entries(&self) -> Vec<String> {
        std::fs::read_dir(self.storage())
            .unwrap()
            .map(|e| e.unwrap().file_name().to_string_lossy().into_owned())
            .collect()
    }

    async fn upload_raw(&self, body: Vec<u8>) -> Response {
        self.client
            .post(format!("{}/files", self.base))
            .header(
                "content-type",
                format!("multipart/form-data; boundary={BOUNDARY}"),
            )
            .body(body)
            .send()
            .await
            .unwrap()
    }

    async fn upload(&self, filename: &str, content_type: Option<&str>, data: &[u8]) -> Response {
        self.upload_raw(multipart("file", Some(filename), content_type, data))
            .await
    }

    /// Uploads, asserts 201 and returns the JSON metadata.
    async fn upload_ok(&self, filename: &str, content_type: Option<&str>, data: &[u8]) -> Value {
        let resp = self.upload(filename, content_type, data).await;
        assert_eq!(resp.status(), StatusCode::CREATED);
        resp.json().await.unwrap()
    }

    async fn get(&self, path: &str) -> Response {
        self.client
            .get(format!("{}{path}", self.base))
            .send()
            .await
            .unwrap()
    }

    async fn get_with_etag(&self, path: &str, if_none_match: &str) -> Response {
        self.client
            .get(format!("{}{path}", self.base))
            .header("if-none-match", if_none_match)
            .send()
            .await
            .unwrap()
    }
}

fn multipart(
    field: &str,
    filename: Option<&str>,
    content_type: Option<&str>,
    data: &[u8],
) -> Vec<u8> {
    let mut disposition = format!("Content-Disposition: form-data; name=\"{field}\"");
    if let Some(filename) = filename {
        disposition.push_str(&format!("; filename=\"{filename}\""));
    }
    let mut body = format!("--{BOUNDARY}\r\n{disposition}\r\n").into_bytes();
    if let Some(content_type) = content_type {
        body.extend_from_slice(format!("Content-Type: {content_type}\r\n").as_bytes());
    }
    body.extend_from_slice(b"\r\n");
    body.extend_from_slice(data);
    body.extend_from_slice(format!("\r\n--{BOUNDARY}--\r\n").as_bytes());
    body
}

fn sha256_hex(data: &[u8]) -> String {
    hex::encode(Sha256::digest(data))
}

fn header(resp: &Response, name: &str) -> String {
    resp.headers()
        .get(name)
        .map_or(String::new(), |v| v.to_str().unwrap().to_owned())
}

fn is_inside(dir: &Path, entry: &Path) -> bool {
    entry
        .canonicalize()
        .is_ok_and(|p| p.starts_with(dir.canonicalize().unwrap()))
}

async fn assert_error(resp: Response, status: StatusCode, code: &str) {
    assert_eq!(resp.status(), status);
    assert!(header(&resp, "content-type").starts_with("application/json"));
    let body: Value = resp.json().await.unwrap();
    assert_eq!(body["error"]["code"], code, "{body}");
}

// ---- upload -------------------------------------------------------------

#[tokio::test]
async fn upload_returns_201_with_id_name_size_hash_and_type() {
    let server = Server::start().await;
    let data = b"hello file service";
    let resp = server.upload("notes.txt", Some("text/plain"), data).await;
    assert_eq!(resp.status(), StatusCode::CREATED);
    let location = header(&resp, "location");
    let meta: Value = resp.json().await.unwrap();

    let id = meta["id"].as_str().unwrap();
    assert_eq!(id.len(), 32, "ids are 32 hex characters: {id}");
    assert!(id.bytes().all(|b| b.is_ascii_hexdigit()));
    assert_eq!(location, format!("/files/{id}"));
    assert_eq!(meta["name"], "notes.txt");
    assert_eq!(meta["size"], data.len());
    assert_eq!(meta["sha256"], sha256_hex(data));
    assert_eq!(meta["content_type"], "text/plain");
}

#[tokio::test]
async fn upload_without_a_content_type_defaults_to_octet_stream() {
    let server = Server::start().await;
    let meta = server.upload_ok("blob", None, b"abc").await;
    assert_eq!(meta["content_type"], "application/octet-stream");
}

#[tokio::test]
async fn upload_accepts_an_empty_file() {
    let server = Server::start().await;
    let meta = server.upload_ok("empty.txt", Some("text/plain"), b"").await;
    assert_eq!(meta["size"], 0);
    assert_eq!(
        meta["sha256"],
        "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
    );
}

#[tokio::test]
async fn upload_assigns_a_distinct_id_to_every_file() {
    let server = Server::start().await;
    let a = server.upload_ok("same.txt", None, b"same").await;
    let b = server.upload_ok("same.txt", None, b"same").await;
    assert_ne!(a["id"], b["id"]);
}

#[tokio::test]
async fn upload_over_the_limit_is_413_payload_too_large() {
    let server = Server::start_with_limit(1000).await;
    let resp = server.upload("big.bin", None, &[7u8; 4000]).await;
    assert_error(resp, StatusCode::PAYLOAD_TOO_LARGE, "payload_too_large").await;
}

#[tokio::test]
async fn upload_exactly_at_the_limit_is_accepted() {
    let server = Server::start_with_limit(1000).await;
    let at_limit = server.upload("ok.bin", None, &[1u8; 1000]).await;
    assert_eq!(at_limit.status(), StatusCode::CREATED);
    let over = server.upload("no.bin", None, &[1u8; 1001]).await;
    assert_eq!(over.status(), StatusCode::PAYLOAD_TOO_LARGE);
}

#[tokio::test]
async fn default_limit_is_5_mib() {
    let server = Server::start().await;
    let ok = server.upload("five.bin", None, &vec![0u8; 5 * MIB]).await;
    assert_eq!(ok.status(), StatusCode::CREATED);
    let too_big = server
        .upload("six.bin", None, &vec![0u8; 5 * MIB + 1])
        .await;
    assert_eq!(too_big.status(), StatusCode::PAYLOAD_TOO_LARGE);
}

#[tokio::test]
async fn rejected_upload_leaves_nothing_on_disk() {
    let server = Server::start_with_limit(1000).await;
    let resp = server.upload("big.bin", None, &[7u8; 4000]).await;
    assert_eq!(resp.status(), StatusCode::PAYLOAD_TOO_LARGE);
    assert_eq!(server.storage_entries(), Vec::<String>::new());
}

#[tokio::test]
async fn upload_without_a_file_field_is_400() {
    let server = Server::start().await;
    let body = multipart("other", Some("x.txt"), None, b"data");
    assert_error(
        server.upload_raw(body).await,
        StatusCode::BAD_REQUEST,
        "bad_request",
    )
    .await;
}

#[tokio::test]
async fn upload_that_is_not_multipart_is_400() {
    let server = Server::start().await;
    let resp = server
        .client
        .post(format!("{}/files", server.base))
        .json(&serde_json::json!({"file": "nope"}))
        .send()
        .await
        .unwrap();
    assert_error(resp, StatusCode::BAD_REQUEST, "bad_request").await;
}

// ---- file names and the storage directory ---------------------------------

#[tokio::test]
async fn traversal_filename_is_sanitized_and_cannot_escape_the_storage_dir() {
    let server = Server::start().await;
    let meta = server.upload_ok("../../etc/passwd", None, b"x").await;
    assert_eq!(meta["name"], "passwd");

    let storage = server.storage();
    let entries = server.storage_entries();
    assert!(!entries.is_empty());
    for entry in &entries {
        assert!(
            !entry.contains("passwd"),
            "client name leaked into a path: {entry}"
        );
        assert!(is_inside(&storage, &storage.join(entry)));
    }
    assert!(!server.root.path().join("etc").exists());
    assert!(!server.root.path().join("passwd").exists());
}

#[tokio::test]
async fn filenames_lose_separators_control_characters_and_dot_dot() {
    let server = Server::start().await;
    let cases = [
        ("C:\\Users\\me\\report.pdf", "report.pdf"),
        ("..\\..\\win.ini", "win.ini"),
        ("tab\there.txt", "tabhere.txt"),
        ("a..b.txt", "a.b.txt"),
        ("/etc/", "file"),
        ("..", "file"),
        ("", "file"),
    ];
    for (given, expected) in cases {
        let meta = server.upload_ok(given, None, b"x").await;
        assert_eq!(meta["name"], expected, "for {given:?}");
    }
}

#[tokio::test]
async fn very_long_filenames_are_truncated() {
    let server = Server::start().await;
    let meta = server.upload_ok(&"n".repeat(500), None, b"x").await;
    assert_eq!(meta["name"].as_str().unwrap().len(), 100);
}

// ---- download ---------------------------------------------------------------

#[tokio::test]
async fn download_returns_the_bytes_with_content_headers() {
    let server = Server::start().await;
    let data: Vec<u8> = (0..=255u8).cycle().take(100_000).collect();
    let meta = server
        .upload_ok("data.bin", Some("application/x-test"), &data)
        .await;
    let id = meta["id"].as_str().unwrap();

    let resp = server.get(&format!("/files/{id}")).await;
    assert_eq!(resp.status(), StatusCode::OK);
    assert_eq!(header(&resp, "content-length"), "100000");
    assert_eq!(header(&resp, "content-type"), "application/x-test");
    assert_eq!(
        header(&resp, "content-disposition"),
        "attachment; filename=\"data.bin\""
    );
    assert_eq!(header(&resp, "etag"), format!("\"{}\"", sha256_hex(&data)));
    assert_eq!(resp.bytes().await.unwrap().as_ref(), data.as_slice());
}

#[tokio::test]
async fn download_header_filename_is_ascii_even_for_unicode_names() {
    let server = Server::start().await;
    let meta = server.upload_ok("résumé.pdf", None, b"cv").await;
    assert_eq!(meta["name"], "résumé.pdf");
    let resp = server
        .get(&format!("/files/{}", meta["id"].as_str().unwrap()))
        .await;
    assert_eq!(resp.status(), StatusCode::OK);
    assert_eq!(
        header(&resp, "content-disposition"),
        "attachment; filename=\"r_sum_.pdf\""
    );
}

#[tokio::test]
async fn matching_if_none_match_gives_304_with_no_body() {
    let server = Server::start().await;
    let meta = server.upload_ok("a.txt", None, b"cache me").await;
    let path = format!("/files/{}", meta["id"].as_str().unwrap());
    let etag = format!("\"{}\"", meta["sha256"].as_str().unwrap());

    let resp = server.get_with_etag(&path, &etag).await;
    assert_eq!(resp.status(), StatusCode::NOT_MODIFIED);
    assert_eq!(header(&resp, "etag"), etag);
    assert!(resp.bytes().await.unwrap().is_empty());
}

#[tokio::test]
async fn if_none_match_understands_weak_validators_lists_and_star() {
    let server = Server::start().await;
    let meta = server.upload_ok("a.txt", None, b"cache me").await;
    let path = format!("/files/{}", meta["id"].as_str().unwrap());
    let etag = format!("\"{}\"", meta["sha256"].as_str().unwrap());

    for value in [
        format!("W/{etag}"),
        format!("\"other\", {etag}"),
        "*".to_owned(),
    ] {
        let resp = server.get_with_etag(&path, &value).await;
        assert_eq!(resp.status(), StatusCode::NOT_MODIFIED, "for {value}");
    }
}

#[tokio::test]
async fn non_matching_if_none_match_still_returns_the_file() {
    let server = Server::start().await;
    let meta = server.upload_ok("a.txt", None, b"cache me").await;
    let path = format!("/files/{}", meta["id"].as_str().unwrap());
    let resp = server.get_with_etag(&path, "\"something-else\"").await;
    assert_eq!(resp.status(), StatusCode::OK);
    assert_eq!(resp.text().await.unwrap(), "cache me");
}

#[tokio::test]
async fn download_of_an_unknown_id_is_404() {
    let server = Server::start().await;
    let resp = server.get(&format!("/files/{}", "0".repeat(32))).await;
    assert_error(resp, StatusCode::NOT_FOUND, "not_found").await;
}

#[tokio::test]
async fn ids_that_look_like_paths_are_404_and_never_read_outside_the_storage_dir() {
    let server = Server::start().await;
    std::fs::write(server.root.path().join("secret.json"), b"top secret").unwrap();
    for id in [
        "..%2Fsecret",
        "..%2F..%2Fsecret",
        "%2e%2e%2fsecret",
        "not-an-id",
    ] {
        let resp = server.get(&format!("/files/{id}")).await;
        assert_eq!(resp.status(), StatusCode::NOT_FOUND, "for {id}");
        let delete = server
            .client
            .delete(format!("{}/files/{id}", server.base))
            .send()
            .await
            .unwrap();
        assert_eq!(delete.status(), StatusCode::NOT_FOUND, "delete {id}");
    }
    assert!(server.root.path().join("secret.json").exists());
}

// ---- delete -----------------------------------------------------------------

#[tokio::test]
async fn delete_returns_204_and_removes_the_file_from_disk() {
    let server = Server::start().await;
    let meta = server.upload_ok("gone.txt", None, b"bye").await;
    let path = format!("/files/{}", meta["id"].as_str().unwrap());

    let resp = server
        .client
        .delete(format!("{}{path}", server.base))
        .send()
        .await
        .unwrap();
    assert_eq!(resp.status(), StatusCode::NO_CONTENT);
    assert_eq!(server.get(&path).await.status(), StatusCode::NOT_FOUND);
    assert_eq!(server.storage_entries(), Vec::<String>::new());
}

#[tokio::test]
async fn delete_of_an_unknown_id_is_404() {
    let server = Server::start().await;
    let url = format!("{}/files/{}", server.base, "a".repeat(32));
    let resp = server.client.delete(url).send().await.unwrap();
    assert_error(resp, StatusCode::NOT_FOUND, "not_found").await;
}
