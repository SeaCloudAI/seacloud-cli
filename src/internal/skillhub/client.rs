use reqwest::blocking::{Client as HttpClient, Response};
use serde::{Deserialize, Serialize};
use std::env;
use std::fs;
use std::io::Read;
use std::path::PathBuf;

fn default_base_url() -> String {
    option_env!("SEACLOUD_SKILLHUB_URL")
        .unwrap_or("")
        .to_string()
}

#[derive(Debug, Clone)]
pub struct Client {
    pub api_base_url: String,
    http_client: HttpClient,
}

impl Client {
    pub fn new() -> Self {
        let mut api_url = env::var("SEACLOUD_SKILLHUB_URL").unwrap_or_default();
        if api_url.is_empty() {
            if let Ok(config) = load_config() {
                if !config.api_base_url.is_empty() {
                    api_url = config.api_base_url;
                } else {
                    api_url = default_base_url();
                }
            } else {
                api_url = default_base_url();
            }
        }
        Self {
            api_base_url: api_url,
            http_client: HttpClient::new(),
        }
    }

    pub fn get(&self, path: &str) -> anyhow::Result<Response> {
        let url = format!("{}{}", self.api_base_url, path);
        self.http_client
            .get(url)
            .send()
            .map_err(|err| anyhow::anyhow!("request failed: {err}"))
    }

    pub fn download_binary(&self, path: &str) -> anyhow::Result<Vec<u8>> {
        let url = format!("{}{}", self.api_base_url, path);
        let mut resp = self
            .http_client
            .get(url)
            .send()
            .map_err(|err| anyhow::anyhow!("download failed: {err}"))?;
        if resp.status() != reqwest::StatusCode::OK {
            anyhow::bail!("download failed: HTTP {}", resp.status().as_u16());
        }
        let mut data = Vec::new();
        resp.read_to_end(&mut data)?;
        Ok(data)
    }

    pub fn get_skill_detail(&self, slug: &str) -> anyhow::Result<SkillDetail> {
        let resp = self.get(&format!("/skills/{slug}"))?;
        if resp.status() == reqwest::StatusCode::NOT_FOUND {
            anyhow::bail!("Skill not found");
        }
        if resp.status() != reqwest::StatusCode::OK {
            anyhow::bail!("Get skill failed: HTTP {}", resp.status().as_u16());
        }
        resp.json()
            .map_err(|err| anyhow::anyhow!("parse response failed: {err}"))
    }

    pub fn download_skill(&self, slug: &str, version: &str) -> anyhow::Result<Vec<u8>> {
        let mut path = format!("/skills/{slug}/download");
        if !version.is_empty() {
            path.push_str("?version=");
            path.push_str(version);
        }
        self.download_binary(&path)
    }

    pub fn search_skills(
        &self,
        query: &str,
        category: &str,
        cursor: &str,
    ) -> anyhow::Result<SearchResult> {
        let mut path = format!("/search?q={query}&limit=20");
        if !category.is_empty() {
            path.push_str("&category=");
            path.push_str(category);
        }
        if !cursor.is_empty() {
            path.push_str("&cursor=");
            path.push_str(cursor);
        }

        let resp = self.get(&path)?;
        let status = resp.status();
        let body = resp.text()?;
        if status != reqwest::StatusCode::OK {
            anyhow::bail!("search failed: HTTP {} - {}", status.as_u16(), body);
        }
        serde_json::from_str(&body).map_err(|err| anyhow::anyhow!("parse response failed: {err}"))
    }
}

impl Default for Client {
    fn default() -> Self {
        Self::new()
    }
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SkillDetail {
    pub skill: SkillInfo,
    #[serde(rename = "latestVersion")]
    pub latest_version: SkillVersion,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SkillInfo {
    #[serde(default)]
    pub slug: String,
    #[serde(rename = "displayName", default)]
    pub display_name: String,
    #[serde(default)]
    pub description: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SkillVersion {
    #[serde(default)]
    pub version: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SearchResult {
    #[serde(default)]
    pub results: Vec<SearchSkill>,
    #[serde(rename = "nextCursor", default)]
    pub next_cursor: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SearchSkill {
    #[serde(default)]
    pub slug: String,
    #[serde(rename = "displayName", default)]
    pub display_name: String,
    #[serde(default)]
    pub description: String,
    #[serde(rename = "updatedAt", default)]
    pub updated_at: i64,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Config {
    #[serde(default)]
    pub api_base_url: String,
    #[serde(default)]
    pub install_dir: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub auth_token: String,
}

fn home_dir() -> PathBuf {
    env::var_os("HOME")
        .map(PathBuf::from)
        .or_else(dirs::home_dir)
        .unwrap_or_else(|| PathBuf::from("."))
}

pub fn config_file_path() -> PathBuf {
    home_dir()
        .join(".claude")
        .join("seacloud-skills-config.json")
}

pub fn load_config() -> anyhow::Result<Config> {
    let path = config_file_path();
    let data = match fs::read_to_string(&path) {
        Ok(data) => data,
        Err(err) if err.kind() == std::io::ErrorKind::NotFound => {
            return Ok(Config {
                api_base_url: default_base_url(),
                install_dir: String::new(),
                auth_token: String::new(),
            });
        }
        Err(err) => return Err(err.into()),
    };
    Ok(serde_json::from_str(&data)?)
}

pub fn save_config(config: &Config) -> anyhow::Result<()> {
    let path = config_file_path();
    if let Some(parent) = path.parent() {
        fs::create_dir_all(parent)?;
    }
    let data = serde_json::to_string_pretty(config)?;
    fs::write(path, data)?;
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::internal::test_support::{TestResponse, TestServer};
    use serial_test::serial;
    use std::env;
    use tempfile::TempDir;

    #[test]
    #[serial]
    fn config_round_trip_and_new_client_priority() {
        let home = TempDir::new().unwrap();
        env::set_var("HOME", home.path());
        env::remove_var("SEACLOUD_SKILLHUB_URL");
        save_config(&Config {
            api_base_url: "https://config.example.com".to_string(),
            install_dir: "dir".to_string(),
            auth_token: "token".to_string(),
        })
        .unwrap();
        let cfg = load_config().unwrap();
        assert_eq!(cfg.api_base_url, "https://config.example.com");
        assert!(config_file_path().ends_with(".claude/seacloud-skills-config.json"));

        assert_eq!(Client::new().api_base_url, "https://config.example.com");
        env::set_var("SEACLOUD_SKILLHUB_URL", "https://env.example.com");
        assert_eq!(Client::new().api_base_url, "https://env.example.com");
    }

    #[test]
    #[serial]
    fn search_detail_and_download_parse_responses() {
        env::remove_var("SEACLOUD_SKILLHUB_URL");
        let server = TestServer::new(|req| match req.path.as_str() {
            "/search?q=cat&limit=20&category=image&cursor=next" => TestResponse::json(
                200,
                r#"{"results":[{"slug":"cat","displayName":"Cat","description":"desc","updatedAt":1}],"nextCursor":"n2"}"#,
            ),
            "/skills/cat" => TestResponse::json(
                200,
                r#"{"skill":{"slug":"cat","displayName":"Cat","description":"desc"},"latestVersion":{"version":"1.0.0"}}"#,
            ),
            "/skills/cat/download?version=1.0.0" => TestResponse {
                status: 200,
                body: b"zip-bytes".to_vec(),
                content_type: "application/octet-stream".to_string(),
            },
            other => panic!("unexpected path {other}"),
        });
        env::set_var("SEACLOUD_SKILLHUB_URL", server.url());
        let client = Client::new();
        let search = client.search_skills("cat", "image", "next").unwrap();
        assert_eq!(search.results[0].slug, "cat");
        assert_eq!(search.next_cursor, "n2");
        let detail = client.get_skill_detail("cat").unwrap();
        assert_eq!(detail.latest_version.version, "1.0.0");
        assert_eq!(client.download_skill("cat", "1.0.0").unwrap(), b"zip-bytes");
    }

    #[test]
    #[serial]
    fn client_reports_http_errors() {
        let server = TestServer::new(|req| match req.path.as_str() {
            "/skills/missing" => TestResponse::json(404, "{}"),
            "/skills/bad/download" => TestResponse::json(500, "bad"),
            "/search?q=bad&limit=20" => TestResponse::json(500, "bad"),
            _ => TestResponse::json(500, "bad"),
        });
        env::set_var("SEACLOUD_SKILLHUB_URL", server.url());
        let client = Client::new();
        assert!(client
            .get_skill_detail("missing")
            .unwrap_err()
            .to_string()
            .contains("Skill not found"));
        assert!(client
            .download_skill("bad", "")
            .unwrap_err()
            .to_string()
            .contains("HTTP 500"));
        assert!(client
            .search_skills("bad", "", "")
            .unwrap_err()
            .to_string()
            .contains("search failed"));
    }
}
