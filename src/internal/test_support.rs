use std::collections::BTreeMap;
use std::io::{Read, Write};
use std::net::{TcpListener, TcpStream};
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::{Arc, Mutex};
use std::thread;
use std::time::Duration;

#[derive(Debug, Clone)]
pub struct CapturedRequest {
    pub method: String,
    pub path: String,
    pub headers: BTreeMap<String, String>,
    pub body: String,
}

#[derive(Clone)]
pub struct TestResponse {
    pub status: u16,
    pub body: Vec<u8>,
    pub content_type: String,
}

impl TestResponse {
    pub fn json(status: u16, body: impl Into<String>) -> Self {
        Self {
            status,
            body: body.into().into_bytes(),
            content_type: "application/json".to_string(),
        }
    }

    pub fn bytes(status: u16, body: impl Into<Vec<u8>>, content_type: &str) -> Self {
        Self {
            status,
            body: body.into(),
            content_type: content_type.to_string(),
        }
    }
}

pub struct TestServer {
    url: String,
    requests: Arc<Mutex<Vec<CapturedRequest>>>,
    stop: Arc<AtomicBool>,
}

impl TestServer {
    pub fn new(handler: impl Fn(CapturedRequest) -> TestResponse + Send + Sync + 'static) -> Self {
        let listener = TcpListener::bind("127.0.0.1:0").expect("bind test server");
        listener
            .set_nonblocking(true)
            .expect("set nonblocking test server");
        let addr = listener.local_addr().expect("test server addr");
        let url = format!("http://{addr}");
        let requests = Arc::new(Mutex::new(Vec::new()));
        let stop = Arc::new(AtomicBool::new(false));
        let handler = Arc::new(handler);
        let thread_requests = Arc::clone(&requests);
        let thread_stop = Arc::clone(&stop);

        thread::spawn(move || {
            while !thread_stop.load(Ordering::SeqCst) {
                match listener.accept() {
                    Ok((stream, _)) => {
                        let requests = Arc::clone(&thread_requests);
                        let handler = Arc::clone(&handler);
                        handle_stream(stream, requests, handler);
                    }
                    Err(err) if err.kind() == std::io::ErrorKind::WouldBlock => {
                        thread::sleep(Duration::from_millis(5));
                    }
                    Err(_) => break,
                }
            }
        });

        Self {
            url,
            requests,
            stop,
        }
    }

    pub fn url(&self) -> String {
        self.url.clone()
    }

    pub fn requests(&self) -> Vec<CapturedRequest> {
        self.requests.lock().expect("requests lock").clone()
    }
}

impl Drop for TestServer {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::SeqCst);
        let _ = TcpStream::connect(self.url.trim_start_matches("http://"));
    }
}

fn handle_stream(
    mut stream: TcpStream,
    requests: Arc<Mutex<Vec<CapturedRequest>>>,
    handler: Arc<dyn Fn(CapturedRequest) -> TestResponse + Send + Sync>,
) {
    let mut buffer = Vec::new();
    let mut temp = [0u8; 4096];
    let _ = stream.set_read_timeout(Some(Duration::from_secs(2)));

    loop {
        let read = match stream.read(&mut temp) {
            Ok(0) => break,
            Ok(read) => read,
            Err(_) => return,
        };
        buffer.extend_from_slice(&temp[..read]);
        if request_complete(&buffer) {
            break;
        }
    }

    let request = parse_request(&buffer);
    requests
        .lock()
        .expect("requests lock")
        .push(request.clone());
    let response = handler(request);
    let status_text = match response.status {
        200 => "OK",
        400 => "Bad Request",
        401 => "Unauthorized",
        403 => "Forbidden",
        404 => "Not Found",
        500 => "Internal Server Error",
        _ => "OK",
    };
    let head = format!(
        "HTTP/1.1 {} {}\r\nContent-Type: {}\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{}",
        response.status,
        status_text,
        response.content_type,
        response.body.len(),
        ""
    );
    let _ = stream.write_all(head.as_bytes());
    let _ = stream.write_all(&response.body);
}

fn request_complete(buffer: &[u8]) -> bool {
    let Some(header_end) = find_header_end(buffer) else {
        return false;
    };
    let headers = String::from_utf8_lossy(&buffer[..header_end]);
    let content_length = headers
        .lines()
        .find_map(|line| {
            let (name, value) = line.split_once(':')?;
            if name.eq_ignore_ascii_case("content-length") {
                value.trim().parse::<usize>().ok()
            } else {
                None
            }
        })
        .unwrap_or(0);
    buffer.len() >= header_end + 4 + content_length
}

fn parse_request(buffer: &[u8]) -> CapturedRequest {
    let header_end = find_header_end(buffer).unwrap_or(buffer.len());
    let head = String::from_utf8_lossy(&buffer[..header_end]);
    let body_start = (header_end + 4).min(buffer.len());
    let mut lines = head.lines();
    let request_line = lines.next().unwrap_or_default();
    let mut request_parts = request_line.split_whitespace();
    let method = request_parts.next().unwrap_or_default().to_string();
    let path = request_parts.next().unwrap_or_default().to_string();
    let mut headers = BTreeMap::new();
    for line in lines {
        if let Some((name, value)) = line.split_once(':') {
            headers.insert(name.trim().to_ascii_lowercase(), value.trim().to_string());
        }
    }
    let body = String::from_utf8_lossy(&buffer[body_start..]).to_string();
    CapturedRequest {
        method,
        path,
        headers,
        body,
    }
}

fn find_header_end(buffer: &[u8]) -> Option<usize> {
    buffer.windows(4).position(|window| window == b"\r\n\r\n")
}
