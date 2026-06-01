use crate::internal::{buildinfo, clierrors};
use anyhow::Context;
use reqwest::blocking::{Client as HttpClient, Response};
use serde::{Deserialize, Serialize};
use serde_json::Value;
use std::time::Duration;

pub const APP_ID: &str = "@seacloud/cli";

fn base_url() -> String {
    std::env::var("SEACLOUD_BASE_URL")
        .ok()
        .filter(|value| !value.trim().is_empty())
        .unwrap_or_else(|| option_env!("SEACLOUD_BASE_URL").unwrap_or("").to_string())
}

#[derive(Debug, Clone)]
pub struct Client {
    http_client: HttpClient,
    token: String,
    base_url: String,
}

impl Client {
    pub fn new(token: impl Into<String>) -> Self {
        Self {
            http_client: HttpClient::builder()
                .timeout(Duration::from_secs(15))
                .build()
                .expect("build HTTP client"),
            token: token.into(),
            base_url: base_url(),
        }
    }

    fn send(&self, req: reqwest::blocking::RequestBuilder) -> anyhow::Result<Response> {
        let mut req = req
            .header("Accept", "*/*")
            .header("Content-Type", "application/json")
            .header("User-Agent", buildinfo::user_agent())
            .header("X-Source", "cli")
            .header("X-App-Id", APP_ID)
            .header("X-Version", buildinfo::VERSION)
            .header("X-Plat", "cli")
            .header("X-Device-Type", "cli")
            .header("X-Skip-Nextauth", "true");
        if !self.token.is_empty() {
            req = req
                .header("Authorization", format!("Bearer {}", self.token))
                .header("X-Auth-Priority", "auth_token");
        }
        req.send().map_err(|err| {
            if err.is_timeout() {
                clierrors::err_network_timeout(err).into()
            } else {
                clierrors::err_network(err).into()
            }
        })
    }

    fn post<B: Serialize, T: for<'de> Deserialize<'de>>(
        &self,
        path: &str,
        body: &B,
    ) -> anyhow::Result<T> {
        let resp = self.send(
            self.http_client
                .post(format!("{}{}", self.base_url, path))
                .json(body),
        )?;
        let text = resp.text()?;
        let envelope: ApiResponse =
            serde_json::from_str(&text).with_context(|| format!("unexpected response: {text}"))?;
        parse_envelope(envelope, &text)
    }

    fn get<T: for<'de> Deserialize<'de>>(&self, path: &str) -> anyhow::Result<T> {
        let resp = self.send(self.http_client.get(format!("{}{}", self.base_url, path)))?;
        let text = resp.text()?;
        match serde_json::from_str::<ApiResponse>(&text) {
            Ok(envelope) => {
                if let Some(data) = envelope.data {
                    parse_status(envelope.status)?;
                    Ok(serde_json::from_value(data)?)
                } else {
                    parse_status(envelope.status)?;
                    Ok(serde_json::from_str(&text)?)
                }
            }
            Err(_) => Ok(serde_json::from_str(&text)?),
        }
    }

    pub fn me(&self) -> anyhow::Result<MeResponse> {
        let data: MeData = self.get("/api/v1/auth/me")?;
        data.user
            .ok_or_else(|| anyhow::anyhow!("user not found in response"))
    }

    pub fn request_device_code(
        &self,
        req: DeviceCodeRequest,
    ) -> anyhow::Result<DeviceCodeResponse> {
        self.post("/api/v1/cli/device/code", &req)
    }

    pub fn poll_token(&self, req: TokenRequest) -> anyhow::Result<TokenResponse> {
        self.post("/api/v1/cli/token", &req)
    }
}

#[derive(Debug, Deserialize)]
struct ApiResponse {
    data: Option<Value>,
    status: ApiStatus,
}

#[derive(Debug, Deserialize)]
struct ApiStatus {
    code: i64,
    #[serde(default)]
    message: String,
}

fn parse_status(status: ApiStatus) -> anyhow::Result<()> {
    if status.code != 0 && status.code != 200 {
        match status.code {
            401 => return Err(clierrors::err_token_expired().into()),
            403 => return Err(clierrors::err_token_invalid().into()),
            _ => anyhow::bail!("{}", status.message),
        }
    }
    Ok(())
}

fn parse_envelope<T: for<'de> Deserialize<'de>>(
    envelope: ApiResponse,
    raw_body: &str,
) -> anyhow::Result<T> {
    parse_status(envelope.status)?;
    let Some(data) = envelope.data else {
        anyhow::bail!("unexpected response: {raw_body}");
    };
    Ok(serde_json::from_value(data)?)
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct MeResponse {
    #[serde(rename = "id", default)]
    pub user_id: String,
    #[serde(default)]
    pub email: String,
    #[serde(default)]
    pub account: String,
    #[serde(default)]
    pub name: String,
    #[serde(default)]
    pub role: String,
}

#[derive(Debug, Deserialize)]
struct MeData {
    user: Option<MeResponse>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct DeviceCodeRequest {
    pub client_id: String,
    pub client_public_key: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct DeviceCodeResponse {
    pub device_code: String,
    pub user_code: String,
    pub verification_uri: String,
    pub expires_in: i64,
    pub interval: i64,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct TokenRequest {
    pub device_code: String,
    pub timestamp: String,
    pub nonce: String,
    pub proof: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct TokenResponse {
    #[serde(default)]
    pub status: String,
    #[serde(default)]
    pub access_token: String,
    #[serde(default)]
    pub refresh_token: String,
    #[serde(default)]
    pub api_key: String,
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::internal::test_support::{TestResponse, TestServer};
    use serial_test::serial;
    use std::env;

    #[test]
    #[serial]
    fn request_device_code_poll_token_and_me_use_headers_and_envelope() {
        env::remove_var("SEACLOUD_BASE_URL");
        let server = TestServer::new(|req| {
            assert_eq!(
                req.headers.get("x-app-id").map(String::as_str),
                Some(APP_ID)
            );
            assert_eq!(req.headers.get("x-source").map(String::as_str), Some("cli"));
            match req.path.as_str() {
                "/api/v1/cli/device/code" => {
                    let body: serde_json::Value = serde_json::from_str(&req.body).unwrap();
                    assert_eq!(body["client_id"], "client");
                    TestResponse::json(
                        200,
                        r#"{"status":{"code":200,"message":"ok"},"data":{"device_code":"dev","user_code":"USER","verification_uri":"https://login","expires_in":60,"interval":1}}"#,
                    )
                }
                "/api/v1/cli/token" => TestResponse::json(
                    200,
                    r#"{"status":{"code":200,"message":"ok"},"data":{"status":"authorized","access_token":"access","refresh_token":"refresh","api_key":"api"}}"#,
                ),
                "/api/v1/auth/me" => {
                    assert_eq!(
                        req.headers.get("authorization").map(String::as_str),
                        Some("Bearer access")
                    );
                    TestResponse::json(
                        200,
                        r#"{"status":{"code":200,"message":"ok"},"data":{"user":{"id":"u","email":"e@example.com","account":"acct","name":"Name","role":"user"}}}"#,
                    )
                }
                other => panic!("unexpected path {other}"),
            }
        });
        env::set_var("SEACLOUD_BASE_URL", server.url());

        let client = Client::new("");
        let dc = client
            .request_device_code(DeviceCodeRequest {
                client_id: "client".to_string(),
                client_public_key: "pub".to_string(),
            })
            .unwrap();
        assert_eq!(dc.device_code, "dev");
        let token = client
            .poll_token(TokenRequest {
                device_code: "dev".to_string(),
                timestamp: "1".to_string(),
                nonce: "n".to_string(),
                proof: "p".to_string(),
            })
            .unwrap();
        assert_eq!(token.api_key, "api");
        let me = Client::new("access").me().unwrap();
        assert_eq!(me.email, "e@example.com");
    }

    #[test]
    #[serial]
    fn status_errors_map_to_cli_errors_and_raw_json_is_supported() {
        env::remove_var("SEACLOUD_BASE_URL");
        let server = TestServer::new(|req| {
            if req.path == "/api/v1/auth/me" {
                return TestResponse::json(200, r#"{"status":{"code":401,"message":"expired"}}"#);
            }
            TestResponse::json(200, "{}")
        });
        env::set_var("SEACLOUD_BASE_URL", server.url());
        let err = Client::new("bad").me().unwrap_err();
        assert!(err.to_string().contains("session expired"));

        let server = TestServer::new(|_| {
            TestResponse::json(
                200,
                r#"{"user":{"id":"u","email":"raw@example.com","account":"acct"}}"#,
            )
        });
        env::set_var("SEACLOUD_BASE_URL", server.url());
        let me = Client::new("token").me().unwrap();
        assert_eq!(me.email, "raw@example.com");
    }
}
