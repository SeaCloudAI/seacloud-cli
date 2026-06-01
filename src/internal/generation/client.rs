use crate::internal::{buildinfo, config};
use reqwest::blocking::Client as HttpClient;
use serde::de::{self, Deserializer};
use serde::{Deserialize, Serialize};
use serde_json::Value;
use std::collections::BTreeMap;
use std::thread;
use std::time::{Duration, Instant};
use url::Url;

pub const DEFAULT_POLL_INTERVAL: Duration = Duration::from_secs(5);

fn base_url() -> String {
    std::env::var("SEACLOUD_GENERATION_URL")
        .ok()
        .filter(|value| !value.trim().is_empty())
        .unwrap_or_else(|| {
            option_env!("SEACLOUD_GENERATION_URL")
                .unwrap_or("")
                .to_string()
        })
}

#[derive(Debug, Clone)]
pub struct Client {
    http_client: HttpClient,
    api_key: String,
}

impl Client {
    pub fn new(api_key: impl Into<String>) -> Self {
        Self {
            http_client: HttpClient::builder()
                .timeout(Duration::from_secs(30))
                .build()
                .expect("build HTTP client"),
            api_key: api_key.into(),
        }
    }

    fn do_json<T: for<'de> Deserialize<'de>>(
        &self,
        method: reqwest::Method,
        endpoint: &str,
        body: Option<Value>,
    ) -> anyhow::Result<T> {
        let mut req = self
            .http_client
            .request(method, endpoint)
            .header("Content-Type", "application/json")
            .header("Authorization", format!("Bearer {}", self.api_key))
            .header("User-Agent", buildinfo::user_agent())
            .header("X-Source", "cli");
        for (key, value) in config::folkos_runtime_headers() {
            req = req.header(key, value);
        }
        if let Some(body) = body {
            req = req.json(&body);
        } else {
            req = req.body(Vec::new());
        }

        let resp = req.send()?;
        let status = resp.status();
        let text = resp.text()?;
        if status.as_u16() >= 400 {
            if let Ok(err_body) = serde_json::from_str::<ApiErrorBody>(&text) {
                let msg = first_non_empty([err_body.message, err_body.error]);
                if !msg.is_empty() {
                    anyhow::bail!("HTTP {}: {}", status.as_u16(), msg);
                }
            }
            anyhow::bail!("HTTP {}: {}", status.as_u16(), text);
        }
        Ok(serde_json::from_str(&text)?)
    }

    pub fn submit(
        &self,
        endpoint: &str,
        model_id: &str,
        params: BTreeMap<String, Value>,
    ) -> anyhow::Result<TaskStatus> {
        let body = serde_json::json!({
            "model": model_id,
            "input": [{"params": params}],
        });
        let resp: TaskStatus = self.do_json(reqwest::Method::POST, endpoint, Some(body))?;
        if resp.id.is_empty() {
            if let Some(error) = resp.error.as_ref().filter(|err| !err.message.is_empty()) {
                anyhow::bail!("{}", error.message);
            }
            anyhow::bail!("task creation failed: no id in response");
        }
        Ok(resp)
    }

    pub fn poll_task(
        &self,
        generation_endpoint: &str,
        task_id: &str,
        poll_interval: Duration,
        timeout: Duration,
        mut on_progress: impl FnMut(f64),
    ) -> anyhow::Result<TaskStatus> {
        let task_endpoint = task_endpoint_from(generation_endpoint, task_id);
        let deadline = Instant::now() + timeout;

        while Instant::now() < deadline {
            let status: TaskStatus = match self.do_json(reqwest::Method::GET, &task_endpoint, None)
            {
                Ok(status) => status,
                Err(_) => {
                    thread::sleep(poll_interval);
                    continue;
                }
            };

            on_progress(status.progress);
            match status.status.as_str() {
                "completed" => return Ok(status),
                "failed" => {
                    let reason = status
                        .error
                        .as_ref()
                        .map(|err| err.message.as_str())
                        .filter(|value| !value.is_empty())
                        .unwrap_or("unknown error");
                    anyhow::bail!("{}", reason);
                }
                _ => thread::sleep(poll_interval),
            }
        }

        anyhow::bail!("timed out after {:?}", timeout)
    }

    pub fn get_task(&self, task_id: &str) -> anyhow::Result<TaskStatus> {
        let base = base_url();
        if base.trim().is_empty() {
            anyhow::bail!("generation base URL not configured: set SEACLOUD_GENERATION_URL or rebuild with -ldflags");
        }
        let endpoint = format!(
            "{}/model/v1/generation/task/{}",
            base.trim_end_matches('/'),
            task_id
        );
        let endpoint = config::rewrite_url_through_folkos_proxy(&endpoint);
        self.do_json(reqwest::Method::GET, &endpoint, None)
    }
}

#[derive(Debug, Deserialize)]
struct ApiErrorBody {
    #[serde(default)]
    message: String,
    #[serde(default)]
    error: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct TaskStatus {
    #[serde(default)]
    pub id: String,
    #[serde(default)]
    pub status: String,
    #[serde(default)]
    pub model: String,
    #[serde(default)]
    pub output: Vec<OutputGroup>,
    #[serde(default)]
    pub error: Option<TaskError>,
    #[serde(default)]
    pub created_at: i64,
    #[serde(default)]
    pub progress: f64,
    #[serde(default)]
    pub usage: Option<UsageInfo>,
}

impl TaskStatus {
    pub fn urls(&self) -> Vec<String> {
        self.output
            .iter()
            .flat_map(|group| &group.content)
            .filter_map(|content| {
                if content.url.is_empty() {
                    None
                } else {
                    Some(content.url.clone())
                }
            })
            .collect()
    }
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct OutputGroup {
    #[serde(default)]
    pub content: Vec<OutputContent>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct OutputContent {
    #[serde(rename = "type", default)]
    pub content_type: String,
    #[serde(default)]
    pub url: String,
    #[serde(default)]
    pub duration: String,
    #[serde(default)]
    pub id: String,
    #[serde(default)]
    pub img_id: i64,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct UsageInfo {
    #[serde(default)]
    pub cost: String,
    #[serde(default)]
    pub quantity: i64,
    #[serde(default)]
    pub unit_price: String,
    #[serde(default)]
    pub extra_info: BTreeMap<String, Value>,
}

#[derive(Debug, Clone, Serialize)]
pub struct TaskError {
    pub message: String,
}

impl<'de> Deserialize<'de> for TaskError {
    fn deserialize<D>(deserializer: D) -> Result<Self, D::Error>
    where
        D: Deserializer<'de>,
    {
        let value = Value::deserialize(deserializer)?;
        if let Some(message) = value.as_str() {
            return Ok(Self {
                message: message.to_string(),
            });
        }
        if let Some(object) = value.as_object() {
            let message = object
                .get("error_message")
                .or_else(|| object.get("message"))
                .and_then(Value::as_str)
                .unwrap_or_default()
                .to_string();
            return Ok(Self { message });
        }
        Err(de::Error::custom("unsupported task error payload"))
    }
}

pub fn task_endpoint_from(generation_endpoint: &str, task_id: &str) -> String {
    match Url::parse(generation_endpoint) {
        Ok(mut url) => {
            let path = format!("{}/task/{}", url.path().trim_end_matches('/'), task_id);
            url.set_path(&path);
            url.to_string()
        }
        Err(_) => format!("{}/task/{}", generation_endpoint, task_id),
    }
}

fn first_non_empty(values: impl IntoIterator<Item = String>) -> String {
    values
        .into_iter()
        .map(|value| value.trim().to_string())
        .find(|value| !value.is_empty())
        .unwrap_or_default()
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::internal::config;
    use crate::internal::test_support::{TestResponse, TestServer};
    use serde_json::json;
    use serial_test::serial;
    use std::env;
    use std::sync::{Arc, Mutex};

    fn clear_env() {
        for key in [
            "SEACLOUD_GENERATION_URL",
            config::ENV_FOLKOS_EXEC_TOKEN,
            config::ENV_FOLKOS_TURN_ID,
            config::ENV_FOLKOS_MESSAGE_ID,
            config::ENV_SEACLOUD_RUNTIME,
            config::ENV_GATEWAY_URL,
            "SEACLOUD_DEFAULT_FOLKOS_PROXY_URL",
        ] {
            env::remove_var(key);
        }
    }

    #[test]
    #[serial]
    fn submit_posts_request_and_parses_task() {
        clear_env();
        env::set_var(config::ENV_FOLKOS_TURN_ID, "turn-1");
        let server = TestServer::new(|req| {
            assert_eq!(req.method, "POST");
            assert_eq!(req.path, "/model/v1/generation");
            assert_eq!(
                req.headers.get("authorization").map(String::as_str),
                Some("Bearer api-key")
            );
            assert_eq!(
                req.headers.get("x-folkos-turn-id").map(String::as_str),
                Some("turn-1")
            );
            let body: Value = serde_json::from_str(&req.body).unwrap();
            assert_eq!(body["model"], "model-1");
            assert_eq!(body["input"][0]["params"]["prompt"], "cat");
            TestResponse::json(
                200,
                r#"{"id":"task-1","status":"in_progress","model":"model-1","created_at":1}"#,
            )
        });

        let mut params = BTreeMap::new();
        params.insert("prompt".to_string(), json!("cat"));
        let task = Client::new("api-key")
            .submit(&(server.url() + "/model/v1/generation"), "model-1", params)
            .unwrap();
        assert_eq!(task.id, "task-1");
    }

    #[test]
    fn task_error_and_urls_parse_multiple_shapes() {
        let task: TaskStatus = serde_json::from_str(
            r#"{"id":"t","status":"failed","error":{"error_message":"bad"},"output":[{"content":[{"type":"image","url":"https://u","img_id":7},{"type":"text"}]}]}"#,
        )
        .unwrap();
        assert_eq!(task.error.as_ref().unwrap().message, "bad");
        assert_eq!(task.urls(), vec!["https://u"]);

        let task: TaskStatus =
            serde_json::from_str(r#"{"id":"t","status":"failed","error":"plain"}"#).unwrap();
        assert_eq!(task.error.unwrap().message, "plain");
    }

    #[test]
    fn task_endpoint_preserves_query_for_url_and_falls_back_for_raw_endpoint() {
        assert_eq!(
            task_endpoint_from("https://example.com/model/v1/generation?debug=1", "task-1"),
            "https://example.com/model/v1/generation/task/task-1?debug=1"
        );
        assert_eq!(
            task_endpoint_from("not-a-url", "task-1"),
            "not-a-url/task/task-1"
        );
    }

    #[test]
    #[serial]
    fn poll_task_handles_progress_completed_failed_and_timeout() {
        clear_env();
        let count = Arc::new(Mutex::new(0usize));
        let server_count = Arc::clone(&count);
        let server = TestServer::new(move |_| {
            let mut count = server_count.lock().unwrap();
            *count += 1;
            if *count == 1 {
                TestResponse::json(200, r#"{"id":"t","status":"in_progress","progress":0.2}"#)
            } else {
                TestResponse::json(
                    200,
                    r#"{"id":"t","status":"completed","progress":1,"output":[{"content":[{"url":"https://done"}]}]}"#,
                )
            }
        });
        let mut progress = Vec::new();
        let task = Client::new("key")
            .poll_task(
                &(server.url() + "/model/v1/generation"),
                "t",
                Duration::from_millis(1),
                Duration::from_secs(1),
                |value| progress.push(value),
            )
            .unwrap();
        assert_eq!(task.status, "completed");
        assert_eq!(progress, vec![0.2, 1.0]);

        let server = TestServer::new(|_| {
            TestResponse::json(200, r#"{"id":"t","status":"failed","error":"bad"}"#)
        });
        let err = Client::new("key")
            .poll_task(
                &(server.url() + "/model/v1/generation"),
                "t",
                Duration::from_millis(1),
                Duration::from_millis(20),
                |_| {},
            )
            .unwrap_err();
        assert_eq!(err.to_string(), "bad");
    }

    #[test]
    #[serial]
    fn get_task_uses_base_url_and_reports_missing_config() {
        clear_env();
        env::remove_var("SEACLOUD_GENERATION_URL");
        let err = Client::new("key").get_task("task-1").unwrap_err();
        assert!(err
            .to_string()
            .contains("generation base URL not configured"));

        let server = TestServer::new(|req| {
            assert_eq!(req.path, "/model/v1/generation/task/task-1");
            TestResponse::json(200, r#"{"id":"task-1","status":"completed"}"#)
        });
        env::set_var("SEACLOUD_GENERATION_URL", server.url());
        let task = Client::new("key").get_task("task-1").unwrap();
        assert_eq!(task.id, "task-1");
    }

    #[test]
    #[serial]
    fn http_errors_prefer_structured_message() {
        clear_env();
        let server = TestServer::new(|_| TestResponse::json(400, r#"{"message":"bad request"}"#));
        let err = Client::new("key")
            .submit(
                &(server.url() + "/model/v1/generation"),
                "m",
                BTreeMap::new(),
            )
            .unwrap_err();
        assert!(err.to_string().contains("HTTP 400: bad request"));
    }
}
