use crate::internal::{buildinfo, clierrors, config};
use reqwest::blocking::Client as HttpClient;
use serde::{Deserialize, Serialize};
use std::time::Duration;
use url::Url;

pub const ENV_PROXY_URL: &str = "SEACLOUD_FOLKOS_PROXY_URL";
pub const ROUTE_GENERATE: &str = "/seacloud-cli-proxy-api/images/generations";
pub const ROUTE_UPLOAD_BASE64: &str = "/internal/assets/upload/base64";
pub const DEFAULT_MODEL: &str = "gpt-image-2";
pub const DEFAULT_SIZE: &str = "1024x1024";
pub const DEFAULT_RESPONSE_FORMAT: &str = "b64_json";
pub const DEFAULT_TIMEOUT: Duration = Duration::from_secs(10 * 60);

#[derive(Debug, Clone)]
pub struct Client {
    http_client: HttpClient,
    api_key: String,
    base_url: String,
}

impl Client {
    #[allow(dead_code)]
    pub fn new(api_key: impl Into<String>) -> Self {
        Self::new_with_timeout(api_key, DEFAULT_TIMEOUT)
    }

    pub fn new_with_timeout(api_key: impl Into<String>, timeout: Duration) -> Self {
        let timeout = if timeout.is_zero() {
            DEFAULT_TIMEOUT
        } else {
            timeout
        };
        Self {
            http_client: HttpClient::builder()
                .timeout(timeout)
                .build()
                .expect("build HTTP client"),
            api_key: api_key.into(),
            base_url: resolve_base_url().trim_end_matches('/').to_string(),
        }
    }

    pub fn generate(&self, req: GenerateRequest) -> anyhow::Result<GenerateResponse> {
        if self.base_url.is_empty() {
            anyhow::bail!(
                "proxy base URL not configured: set {ENV_PROXY_URL} or rebuild with -ldflags"
            );
        }
        let resp: GenerateResponse = self.do_json(
            reqwest::Method::POST,
            &(self.base_url.clone() + ROUTE_GENERATE),
            &req,
        )?;
        if resp.data.is_empty() {
            anyhow::bail!("image generation returned no data");
        }
        Ok(resp)
    }

    pub fn upload_base64(
        &self,
        data: &str,
        mime_type_hint: &str,
    ) -> anyhow::Result<Base64UploadResponse> {
        if self.base_url.is_empty() {
            anyhow::bail!(
                "proxy base URL not configured: set {ENV_PROXY_URL} or rebuild with -ldflags"
            );
        }
        let req = Base64UploadRequest {
            data: data.to_string(),
            mime_type_hint: mime_type_hint.trim().to_string(),
        };
        let resp: Base64UploadResponse = self.do_json(
            reqwest::Method::POST,
            &(self.base_url.clone() + ROUTE_UPLOAD_BASE64),
            &req,
        )?;
        if resp.cdn_url.trim().is_empty() {
            anyhow::bail!("assets upload returned no cdn_url");
        }
        Ok(resp)
    }

    pub fn upload_response_images(&self, resp: &GenerateResponse) -> anyhow::Result<Vec<String>> {
        let ext = resp
            .output_format
            .trim()
            .trim_start_matches('.')
            .to_ascii_lowercase();
        let mime_type_hint = mime_guess::from_ext(&ext)
            .first_raw()
            .unwrap_or("")
            .to_string();
        let mut urls = Vec::with_capacity(resp.data.len());
        for item in &resp.data {
            if !item.url.trim().is_empty() {
                urls.push(item.url.trim().to_string());
                continue;
            }
            if item.b64_json.is_empty() {
                continue;
            }
            let uploaded = self.upload_base64(&item.b64_json, &mime_type_hint)?;
            urls.push(uploaded.cdn_url);
        }
        if urls.is_empty() {
            anyhow::bail!("image generation returned no URL or b64_json payload");
        }
        Ok(urls)
    }

    fn do_json<T: for<'de> Deserialize<'de>, B: Serialize>(
        &self,
        method: reqwest::Method,
        endpoint: &str,
        body: &B,
    ) -> anyhow::Result<T> {
        let mut req = self
            .http_client
            .request(method, endpoint)
            .header("Content-Type", "application/json")
            .header("Authorization", format!("Bearer {}", self.api_key))
            .header("User-Agent", buildinfo::user_agent())
            .header("X-Source", "cli")
            .json(body);
        for (key, value) in config::folkos_runtime_headers() {
            req = req.header(key, value);
        }
        let resp = req.send()?;
        let status = resp.status();
        let text = resp.text()?;
        if status.as_u16() >= 400 {
            if let Ok(err_body) = serde_json::from_str::<ApiErrorBody>(&text) {
                let msg = [err_body.message, err_body.error, err_body.code]
                    .into_iter()
                    .map(|value| value.trim().to_string())
                    .find(|value| !value.is_empty())
                    .unwrap_or_default();
                if !msg.is_empty() {
                    anyhow::bail!("HTTP {}: {}", status.as_u16(), msg);
                }
            }
            anyhow::bail!("HTTP {}: {}", status.as_u16(), text);
        }
        Ok(serde_json::from_str(&text)?)
    }
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct GenerateRequest {
    pub model: String,
    pub prompt: String,
    #[serde(skip_serializing_if = "String::is_empty", default)]
    pub size: String,
    #[serde(
        rename = "response_format",
        skip_serializing_if = "String::is_empty",
        default
    )]
    pub response_format: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct GenerateResponse {
    #[serde(default)]
    pub created: i64,
    #[serde(default)]
    pub data: Vec<ImageData>,
    #[serde(default)]
    pub background: String,
    #[serde(default)]
    pub output_format: String,
    #[serde(default)]
    pub quality: String,
    #[serde(default)]
    pub size: String,
    #[serde(default)]
    pub usage: Option<serde_json::Value>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ImageData {
    #[serde(default)]
    pub b64_json: String,
    #[serde(default)]
    pub url: String,
    #[serde(default)]
    pub revised_prompt: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Base64UploadRequest {
    pub data: String,
    #[serde(skip_serializing_if = "String::is_empty", default)]
    pub mime_type_hint: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Base64UploadResponse {
    #[serde(default)]
    pub object_path: String,
    #[serde(default)]
    pub cdn_url: String,
    #[serde(default)]
    pub content_type: String,
    #[serde(default)]
    pub resource_type: String,
    #[serde(default)]
    pub size_bytes: i64,
    #[serde(default)]
    pub sha256: String,
}

#[derive(Debug, Deserialize)]
struct ApiErrorBody {
    #[serde(default)]
    code: String,
    #[serde(default)]
    message: String,
    #[serde(default)]
    error: String,
}

pub fn supports_sync_model(model_id: &str) -> bool {
    model_id
        .trim()
        .to_ascii_lowercase()
        .starts_with("gpt-image")
}

pub fn request_from_values(
    model_id: &str,
    prompt: &str,
    size: &str,
    response_format: &str,
) -> anyhow::Result<GenerateRequest> {
    let model_id = model_id.trim();
    let prompt = prompt.trim();
    let size = size.trim();
    let response_format = response_format.trim();

    let model = if model_id.is_empty() {
        DEFAULT_MODEL
    } else {
        model_id
    };
    if prompt.is_empty() {
        return Err(clierrors::err_missing_param(model, "prompt").into());
    }
    let size = if size.is_empty() { DEFAULT_SIZE } else { size };
    let response_format = if response_format.is_empty() {
        DEFAULT_RESPONSE_FORMAT
    } else {
        response_format
    };
    if response_format != DEFAULT_RESPONSE_FORMAT {
        return Err(clierrors::err_invalid_param(
            model,
            "response_format",
            format!("only {DEFAULT_RESPONSE_FORMAT:?} is supported"),
        )
        .into());
    }

    Ok(GenerateRequest {
        model: model.to_string(),
        prompt: prompt.to_string(),
        size: size.to_string(),
        response_format: response_format.to_string(),
    })
}

pub fn request_from_params(
    model_id: &str,
    raw: &std::collections::BTreeMap<String, String>,
) -> anyhow::Result<GenerateRequest> {
    for name in raw.keys() {
        if name != "prompt" && name != "size" && name != "response_format" {
            return Err(clierrors::err_invalid_param(
                model_id,
                name,
                "only prompt, size, and response_format are supported for sync image generation",
            )
            .into());
        }
    }
    request_from_values(
        model_id,
        raw.get("prompt").map(String::as_str).unwrap_or_default(),
        raw.get("size").map(String::as_str).unwrap_or_default(),
        raw.get("response_format")
            .map(String::as_str)
            .unwrap_or_default(),
    )
}

pub fn summary(resp: &GenerateResponse) -> String {
    let mut lines = vec![format!("Images: {}", resp.data.len())];
    if !resp.output_format.is_empty() {
        lines.push(format!("Output format: {}", resp.output_format));
    }
    if !resp.size.is_empty() {
        lines.push(format!("Size: {}", resp.size));
    }
    for (i, item) in resp.data.iter().enumerate() {
        let index = i + 1;
        if !item.url.trim().is_empty() {
            lines.push(format!("Image {index} URL: {}", item.url));
        } else if !item.b64_json.is_empty() {
            lines.push(format!(
                "Image {index} b64_json length: {}",
                item.b64_json.len()
            ));
        } else {
            lines.push(format!("Image {index}: empty payload"));
        }
        if !item.revised_prompt.is_empty() {
            lines.push(format!(
                "Image {index} revised prompt: {}",
                item.revised_prompt
            ));
        }
    }
    lines.join("\n")
}

fn resolve_base_url() -> String {
    if let Some(env) = normalize_base_url(std::env::var(ENV_PROXY_URL).unwrap_or_default()) {
        return env;
    }
    let folkos_base = config::folkos_proxy_base_url()
        .trim()
        .trim_end_matches('/')
        .to_string();
    if folkos_base.is_empty() {
        return normalize_base_url(option_env!("SEACLOUD_FOLKOS_PROXY_URL").unwrap_or(""))
            .unwrap_or_default();
    }
    folkos_base
        .strip_suffix("/folkos-proxy")
        .unwrap_or(&folkos_base)
        .to_string()
}

fn normalize_base_url(raw: impl AsRef<str>) -> Option<String> {
    let raw = raw.as_ref().trim();
    if raw.is_empty() {
        return None;
    }
    let mut url = Url::parse(raw).ok()?;
    if url.scheme().is_empty() || url.host_str().is_none() {
        return None;
    }
    url.set_query(None);
    url.set_fragment(None);
    let path = url.path().trim_end_matches('/').to_string();
    url.set_path(&path);
    Some(url.to_string().trim_end_matches('/').to_string())
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::internal::config;
    use crate::internal::test_support::{TestResponse, TestServer};
    use serial_test::serial;
    use std::collections::BTreeMap;
    use std::env;

    #[test]
    fn supports_sync_models() {
        assert!(supports_sync_model("gpt-image-2"));
        assert!(!supports_sync_model("kirin_v2_6_i2v"));
    }

    #[test]
    fn request_from_values_defaults_response_format() {
        let req = request_from_values("gpt-image-2", "cat", "", "").unwrap();
        assert_eq!(req.size, DEFAULT_SIZE);
        assert_eq!(req.response_format, DEFAULT_RESPONSE_FORMAT);
    }

    fn clear_env() {
        for key in [
            ENV_PROXY_URL,
            config::ENV_GATEWAY_URL,
            config::ENV_FOLKOS_EXEC_TOKEN,
            config::ENV_SEACLOUD_RUNTIME,
            config::ENV_FOLKOS_TURN_ID,
            config::ENV_FOLKOS_MESSAGE_ID,
        ] {
            env::remove_var(key);
        }
    }

    #[test]
    fn request_validation_rejects_missing_prompt_bad_format_and_unknown_param() {
        assert!(request_from_values("gpt-image-2", "", "", "").is_err());
        assert!(request_from_values("gpt-image-2", "cat", "", "url").is_err());
        let raw = BTreeMap::from([("unknown".to_string(), "x".to_string())]);
        assert!(request_from_params("gpt-image-2", &raw).is_err());
    }

    #[test]
    #[serial]
    fn generate_posts_to_proxy_and_uploads_b64_response() {
        clear_env();
        env::set_var(config::ENV_FOLKOS_TURN_ID, "turn-1");
        let server = TestServer::new(|req| {
            if req.path == ROUTE_GENERATE {
                assert_eq!(req.method, "POST");
                assert_eq!(
                    req.headers.get("authorization").map(String::as_str),
                    Some("Bearer api-key")
                );
                assert_eq!(
                    req.headers.get("x-folkos-turn-id").map(String::as_str),
                    Some("turn-1")
                );
                let body: serde_json::Value = serde_json::from_str(&req.body).unwrap();
                assert_eq!(body["model"], "gpt-image-2");
                assert_eq!(body["prompt"], "cat");
                return TestResponse::json(
                    200,
                    r#"{"data":[{"b64_json":"abc","revised_prompt":"cat!"}],"output_format":"png","size":"1024x1024"}"#,
                );
            }
            assert_eq!(req.path, ROUTE_UPLOAD_BASE64);
            let body: serde_json::Value = serde_json::from_str(&req.body).unwrap();
            assert_eq!(body["data"], "abc");
            assert_eq!(body["mime_type_hint"], "image/png");
            TestResponse::json(
                200,
                r#"{"cdn_url":"https://assets.example.com/cat.png","content_type":"image/png"}"#,
            )
        });
        env::set_var(ENV_PROXY_URL, server.url());

        let client = Client::new_with_timeout("api-key", Duration::from_millis(0));
        let resp = client
            .generate(request_from_values("gpt-image-2", "cat", "", "").unwrap())
            .unwrap();
        assert!(summary(&resp).contains("Image 1 b64_json length: 3"));
        assert!(summary(&resp).contains("Image 1 revised prompt: cat!"));
        let urls = client.upload_response_images(&resp).unwrap();
        assert_eq!(urls, vec!["https://assets.example.com/cat.png"]);
    }

    #[test]
    #[serial]
    fn upload_response_images_uses_existing_url_and_errors_on_empty_payload() {
        clear_env();
        let server = TestServer::new(|_| TestResponse::json(200, "{}"));
        env::set_var(ENV_PROXY_URL, server.url());
        let client = Client::new_with_timeout("api-key", Duration::from_secs(1));
        let urls = client
            .upload_response_images(&GenerateResponse {
                data: vec![ImageData {
                    url: " https://cdn.example.com/u.png ".to_string(),
                    ..empty_image_data()
                }],
                ..empty_generate_response()
            })
            .unwrap();
        assert_eq!(urls, vec!["https://cdn.example.com/u.png"]);

        let err = client
            .upload_response_images(&GenerateResponse {
                data: vec![ImageData {
                    ..empty_image_data()
                }],
                ..empty_generate_response()
            })
            .unwrap_err();
        assert!(err.to_string().contains("no URL or b64_json"));
    }

    #[test]
    #[serial]
    fn generate_and_upload_report_proxy_and_http_errors() {
        clear_env();
        env::remove_var(ENV_PROXY_URL);
        let err = Client::new_with_timeout("api-key", Duration::from_secs(1))
            .generate(request_from_values("gpt-image-2", "cat", "", "").unwrap())
            .unwrap_err();
        assert!(err.to_string().contains("proxy base URL not configured"));

        let server = TestServer::new(|_| TestResponse::json(400, r#"{"error":"bad"}"#));
        env::set_var(ENV_PROXY_URL, server.url());
        let err = Client::new_with_timeout("api-key", Duration::from_secs(1))
            .generate(request_from_values("gpt-image-2", "cat", "", "").unwrap())
            .unwrap_err();
        assert!(err.to_string().contains("HTTP 400: bad"));

        let server = TestServer::new(|_| TestResponse::json(200, r#"{"data":[]}"#));
        env::set_var(ENV_PROXY_URL, server.url());
        let err = Client::new_with_timeout("api-key", Duration::from_secs(1))
            .generate(request_from_values("gpt-image-2", "cat", "", "").unwrap())
            .unwrap_err();
        assert!(err.to_string().contains("returned no data"));
    }

    #[test]
    #[serial]
    fn resolve_base_url_can_use_gateway_runtime() {
        clear_env();
        env::set_var(config::ENV_GATEWAY_URL, "https://gateway.example.com");
        env::set_var(config::ENV_FOLKOS_EXEC_TOKEN, "exec");
        env::set_var(config::ENV_SEACLOUD_RUNTIME, config::RUNTIME_FOLKOS);
        assert_eq!(resolve_base_url(), "https://gateway.example.com");
    }

    fn empty_generate_response() -> GenerateResponse {
        GenerateResponse {
            created: 0,
            data: Vec::new(),
            background: String::new(),
            output_format: String::new(),
            quality: String::new(),
            size: String::new(),
            usage: None,
        }
    }

    fn empty_image_data() -> ImageData {
        ImageData {
            b64_json: String::new(),
            url: String::new(),
            revised_prompt: String::new(),
        }
    }
}
