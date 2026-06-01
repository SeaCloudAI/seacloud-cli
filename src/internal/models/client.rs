use crate::internal::{buildinfo, config};
use anyhow::Context;
use reqwest::blocking::Client as HttpClient;
use serde::{Deserialize, Serialize};
use serde_json::Value;
use std::collections::BTreeMap;
use std::time::Duration;
use url::form_urlencoded;

const DEFAULT_MODELS_TIMEOUT: Duration = Duration::from_secs(30);

fn build_base_url() -> String {
    let base = std::env::var("SEACLOUD_MODELS_URL")
        .ok()
        .filter(|value| !value.trim().is_empty())
        .unwrap_or_else(|| option_env!("SEACLOUD_MODELS_URL").unwrap_or("").to_string());
    config::rewrite_url_through_folkos_proxy(&base)
}

#[derive(Debug, Clone)]
pub struct Client {
    http_client: HttpClient,
    base_url: String,
    auth_token: String,
}

impl Client {
    pub fn new() -> Self {
        Self {
            http_client: HttpClient::builder()
                .timeout(DEFAULT_MODELS_TIMEOUT)
                .build()
                .expect("build HTTP client"),
            base_url: build_base_url(),
            auth_token: config::exec_token_from_env(),
        }
    }

    fn get<T: for<'de> Deserialize<'de>>(&self, path: &str) -> anyhow::Result<T> {
        if self.base_url.is_empty() {
            anyhow::bail!(
                "models base URL not configured: set SEACLOUD_MODELS_URL or rebuild with -ldflags"
            );
        }
        let url = format!("{}{}", self.base_url.trim_end_matches('/'), path);
        let mut req = self
            .http_client
            .get(url)
            .header("User-Agent", buildinfo::user_agent())
            .header("X-Source", "cli");
        if !self.auth_token.is_empty() {
            req = req.header("Authorization", format!("Bearer {}", self.auth_token));
        }

        let body = req.send()?.text()?;
        let envelope: AuthApiResponse =
            serde_json::from_str(&body).with_context(|| format!("unexpected response: {body}"))?;

        if envelope.status.code != 0 && envelope.status.code != 200 {
            anyhow::bail!(
                "status {}: {}",
                envelope.status.code,
                envelope.status.message
            );
        }
        let Some(data) = envelope.data else {
            anyhow::bail!("unexpected response: {body}");
        };
        Ok(serde_json::from_value(data)?)
    }

    pub fn list(&self, params: ListParams) -> anyhow::Result<ModelsListResponse> {
        let query = build_query(&params);
        let path = if query.is_empty() {
            "/api/v1/skill/models".to_string()
        } else {
            format!("/api/v1/skill/models?{query}")
        };
        self.get(&path)
    }

    pub fn get_spec(&self, model_id: &str) -> anyhow::Result<ModelSpec> {
        let mut spec: ModelSpec = self.get(&format!("/api/v1/skill/models/{model_id}/spec"))?;
        let rewritten = config::rewrite_url_through_folkos_proxy(&spec.api.endpoint);
        if rewritten != spec.api.endpoint {
            spec.agent_prompt = spec.agent_prompt.replace(&spec.api.endpoint, &rewritten);
            spec.api.endpoint = rewritten;
        }
        Ok(spec)
    }
}

impl Default for Client {
    fn default() -> Self {
        Self::new()
    }
}

#[derive(Debug, Clone, Default)]
pub struct ListParams {
    pub page: usize,
    pub page_size: usize,
    pub model_type: String,
    pub keywords: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Model {
    pub id: String,
    pub name: String,
    #[serde(rename = "type")]
    pub model_type: String,
    #[serde(default)]
    pub description: String,
    #[serde(default)]
    pub input_modalities: Vec<String>,
    #[serde(default)]
    pub output_modalities: Vec<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ModelsListResponse {
    #[serde(default)]
    pub models: Vec<Model>,
    #[serde(default)]
    pub total: usize,
    #[serde(default)]
    pub page: usize,
    #[serde(default)]
    pub page_size: usize,
    #[serde(default)]
    pub total_pages: usize,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ModelSpec {
    #[serde(default)]
    pub model_id: String,
    #[serde(default)]
    pub name: String,
    #[serde(default)]
    pub vendor: String,
    #[serde(rename = "type", default)]
    pub model_type: String,
    #[serde(default)]
    pub api: ModelSpecApi,
    #[serde(default)]
    pub parameters: Vec<ModelParam>,
    #[serde(default)]
    pub agent_prompt: String,
}

#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct ModelSpecApi {
    #[serde(default)]
    pub endpoint: String,
    #[serde(default)]
    pub method: String,
    #[serde(default)]
    pub headers: BTreeMap<String, String>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ModelParam {
    #[serde(default)]
    pub name: String,
    #[serde(default)]
    pub path: String,
    #[serde(rename = "type", default)]
    pub param_type: String,
    #[serde(default)]
    pub required: bool,
    #[serde(default)]
    pub description: String,
    #[serde(default)]
    pub constraints: Option<ParamConstraints>,
    #[serde(default)]
    pub example: Option<Value>,
    #[serde(default)]
    pub children: Vec<ModelParam>,
}

#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct ParamConstraints {
    #[serde(rename = "enum", default)]
    pub enum_values: Vec<String>,
    #[serde(default)]
    pub default: Option<Value>,
    #[serde(default)]
    pub min: Option<f64>,
    #[serde(default)]
    pub max: Option<f64>,
    #[serde(default)]
    pub min_length: Option<usize>,
    #[serde(default)]
    pub max_length: Option<usize>,
    #[serde(default)]
    pub max_items: Option<usize>,
}

#[derive(Debug, Deserialize)]
struct AuthApiResponse {
    data: Option<Value>,
    status: AuthApiStatus,
}

#[derive(Debug, Deserialize)]
struct AuthApiStatus {
    code: i64,
    #[serde(default)]
    message: String,
}

pub fn build_query(params: &ListParams) -> String {
    let mut ser = form_urlencoded::Serializer::new(String::new());
    if params.page > 0 {
        ser.append_pair("page", &params.page.to_string());
    }
    if params.page_size > 0 {
        ser.append_pair("page_size", &params.page_size.to_string());
    }
    if !params.model_type.is_empty() {
        ser.append_pair("type", &params.model_type);
    }
    if !params.keywords.is_empty() {
        ser.append_pair("keywords", &params.keywords);
    }
    ser.finish()
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::internal::config;
    use crate::internal::test_support::{TestResponse, TestServer};
    use serial_test::serial;
    use std::env;

    fn clear_env() {
        for key in [
            "SEACLOUD_MODELS_URL",
            config::ENV_FOLKOS_EXEC_TOKEN,
            config::ENV_SEACLOUD_RUNTIME,
            config::ENV_GATEWAY_URL,
            "SEACLOUD_DEFAULT_FOLKOS_PROXY_URL",
        ] {
            env::remove_var(key);
        }
    }

    #[test]
    fn query_builder_encodes_filters() {
        assert_eq!(
            build_query(&ListParams {
                page: 2,
                page_size: 10,
                model_type: "video".to_string(),
                keywords: "blue cat".to_string(),
            }),
            "page=2&page_size=10&type=video&keywords=blue+cat"
        );
        assert_eq!(build_query(&ListParams::default()), "");
    }

    #[test]
    #[serial]
    fn list_adds_managed_auth_header_and_parses_response() {
        clear_env();
        env::set_var(config::ENV_FOLKOS_EXEC_TOKEN, "exec-token");
        let server = TestServer::new(|req| {
            assert_eq!(req.method, "GET");
            assert_eq!(req.path, "/api/v1/skill/models?page=1&page_size=1");
            assert_eq!(
                req.headers.get("authorization").map(String::as_str),
                Some("Bearer exec-token")
            );
            TestResponse::json(
                200,
                r#"{"status":{"code":200,"message":"ok"},"data":{"models":[{"id":"kirin_v2_6_i2v","name":"Kling","type":"video","description":"desc","input_modalities":["image"],"output_modalities":["video"]}],"total":1,"page":1,"page_size":1,"total_pages":1}}"#,
            )
        });
        env::set_var("SEACLOUD_MODELS_URL", server.url());

        let result = Client::new()
            .list(ListParams {
                page: 1,
                page_size: 1,
                ..ListParams::default()
            })
            .unwrap();
        assert_eq!(result.total, 1);
        assert_eq!(result.models[0].id, "kirin_v2_6_i2v");
    }

    #[test]
    #[serial]
    fn get_spec_rewrites_vtrix_endpoint_through_proxy() {
        clear_env();
        env::set_var(config::ENV_SEACLOUD_RUNTIME, config::RUNTIME_FOLKOS);
        env::set_var(
            "SEACLOUD_DEFAULT_FOLKOS_PROXY_URL",
            "https://gateway.example.com/folkos-proxy",
        );
        let server = TestServer::new(|req| {
            assert_eq!(req.path, "/api/v1/skill/models/gpt_image_1/spec");
            TestResponse::json(
                200,
                r#"{"status":{"code":200,"message":"ok"},"data":{"model_id":"gpt_image_1","name":"GPT","vendor":"openai","type":"image","api":{"endpoint":"https://cloud.vtrix.ai/model/v1/generation","method":"POST","headers":{}},"parameters":[],"agent_prompt":"POST https://cloud.vtrix.ai/model/v1/generation"}}"#,
            )
        });
        env::set_var("SEACLOUD_MODELS_URL", server.url());

        let spec = Client::new().get_spec("gpt_image_1").unwrap();
        assert_eq!(
            spec.api.endpoint,
            "https://gateway.example.com/folkos-proxy/model/v1/generation"
        );
        assert!(spec.agent_prompt.contains("gateway.example.com"));
    }

    #[test]
    #[serial]
    fn client_reports_api_and_decode_errors() {
        clear_env();
        let server = TestServer::new(|_| {
            TestResponse::json(200, r#"{"status":{"code":404,"message":"missing"}}"#)
        });
        env::set_var("SEACLOUD_MODELS_URL", server.url());
        let err = Client::new().list(ListParams::default()).unwrap_err();
        assert!(err.to_string().contains("status 404"));

        let server = TestServer::new(|_| TestResponse::json(200, "not-json"));
        env::set_var("SEACLOUD_MODELS_URL", server.url());
        let err = Client::new().list(ListParams::default()).unwrap_err();
        assert!(err.to_string().contains("unexpected response"));
    }
}
