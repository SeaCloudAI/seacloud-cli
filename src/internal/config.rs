use serde::{Deserialize, Serialize};
use std::collections::BTreeMap;
use std::env;
use std::fs;
use std::path::PathBuf;
use url::Url;

const KEYCHAIN_SERVICE: &str = "seacloud-cli";

pub const ENV_FOLKOS_EXEC_TOKEN: &str = "FOLKOS_EXEC_TOKEN";
pub const ENV_FOLKOS_TOKEN: &str = "FOLKOS_TOKEN";
pub const ENV_FOLKOS_SESSION_ID: &str = "FOLKOS_SESSION_ID";
pub const ENV_FOLKOS_TURN_ID: &str = "FOLKOS_TURN_ID";
pub const ENV_FOLKOS_MESSAGE_ID: &str = "FOLKOS_MESSAGE_ID";
pub const ENV_FOLKOS_AGENT_UUID: &str = "FOLKOS_AGENT_UUID";
pub const ENV_FOLKOS_WORKSPACE_ID: &str = "FOLKOS_WORKSPACE_ID";
pub const ENV_FOLKOS_AGENT_TEMP_ID: &str = "FOLKOS_AGENT_TEMP_ID";
pub const ENV_FOLKOS_SANDBOX_ID: &str = "FOLKOS_SANDBOX_ID";
pub const ENV_SEACLOUD_RUNTIME: &str = "SEACLOUD_RUNTIME";
pub const ENV_GATEWAY_URL: &str = "GATEWAY_URL";
pub const RUNTIME_FOLKOS: &str = "folkos";

#[derive(Debug, Default, Clone)]
pub struct Config {
    pub auth_token: String,
    pub refresh_token: String,
    pub api_key: String,
    pub managed: bool,
    pub runtime: String,
    pub credential_source: String,
}

#[derive(Debug, Default, Serialize, Deserialize)]
struct FileConfig {}

#[derive(Debug, Default, Serialize, Deserialize)]
struct FileTokens {
    #[serde(default, skip_serializing_if = "String::is_empty")]
    auth_token: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    refresh_token: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    api_key: String,
}

trait TokenStore {
    fn load_tokens(&self) -> anyhow::Result<(String, String, String)>;
    fn save_tokens(&self, auth: &str, refresh: &str, api_key: &str) -> anyhow::Result<()>;
    fn clear_tokens(&self) -> anyhow::Result<()>;
}

struct FileStore;
struct KeychainStore;

impl TokenStore for FileStore {
    fn load_tokens(&self) -> anyhow::Result<(String, String, String)> {
        let Some(tokens) = read_file_tokens() else {
            return Ok((String::new(), String::new(), String::new()));
        };
        Ok((tokens.auth_token, tokens.refresh_token, tokens.api_key))
    }

    fn save_tokens(&self, auth: &str, refresh: &str, api_key: &str) -> anyhow::Result<()> {
        let path = config_path()?;
        if let Some(parent) = path.parent() {
            fs::create_dir_all(parent)?;
        }
        let tokens = FileTokens {
            auth_token: auth.to_string(),
            refresh_token: refresh.to_string(),
            api_key: api_key.to_string(),
        };
        let data = serde_yaml::to_string(&tokens)?;
        write_private_file(&path, data.as_bytes())?;
        Ok(())
    }

    fn clear_tokens(&self) -> anyhow::Result<()> {
        let path = config_path()?;
        match fs::remove_file(path) {
            Ok(()) => Ok(()),
            Err(err) if err.kind() == std::io::ErrorKind::NotFound => Ok(()),
            Err(err) => Err(err.into()),
        }
    }
}

impl TokenStore for KeychainStore {
    fn load_tokens(&self) -> anyhow::Result<(String, String, String)> {
        let auth = match keyring_get("auth_token") {
            Ok(value) => value,
            Err(_) => {
                if let Some(tokens) = legacy_file_tokens() {
                    if self.save_tokens(&tokens[0], &tokens[1], &tokens[2]).is_ok() {
                        let _ = clear_legacy_file_tokens();
                        return Ok((tokens[0].clone(), tokens[1].clone(), tokens[2].clone()));
                    }
                }
                return Ok((String::new(), String::new(), String::new()));
            }
        };
        let refresh = keyring_get("refresh_token").unwrap_or_default();
        let api_key = keyring_get("api_key").unwrap_or_default();
        Ok((auth, refresh, api_key))
    }

    fn save_tokens(&self, auth: &str, refresh: &str, api_key: &str) -> anyhow::Result<()> {
        if let Err(err) = keyring_set("auth_token", auth) {
            eprintln!("warning: keychain write failed ({err}), falling back to file storage");
            return FileStore.save_tokens(auth, refresh, api_key);
        }
        if !refresh.is_empty() {
            let _ = keyring_set("refresh_token", refresh);
        }
        if !api_key.is_empty() {
            let _ = keyring_set("api_key", api_key);
        }
        Ok(())
    }

    fn clear_tokens(&self) -> anyhow::Result<()> {
        let _ = keyring_delete("auth_token");
        let _ = keyring_delete("refresh_token");
        let _ = keyring_delete("api_key");
        Ok(())
    }
}

fn token_store() -> Box<dyn TokenStore> {
    if env::var("SEACLOUD_NO_KEYCHAIN").ok().as_deref() == Some("1") {
        return Box::new(FileStore);
    }
    if cfg!(target_os = "linux")
        && env::var("DBUS_SESSION_BUS_ADDRESS")
            .unwrap_or_default()
            .is_empty()
    {
        return Box::new(FileStore);
    }
    Box::new(KeychainStore)
}

fn keyring_get(name: &str) -> anyhow::Result<String> {
    let entry = keyring::Entry::new(KEYCHAIN_SERVICE, name)?;
    Ok(entry.get_password()?)
}

fn keyring_set(name: &str, value: &str) -> anyhow::Result<()> {
    let entry = keyring::Entry::new(KEYCHAIN_SERVICE, name)?;
    Ok(entry.set_password(value)?)
}

fn keyring_delete(name: &str) -> anyhow::Result<()> {
    let entry = keyring::Entry::new(KEYCHAIN_SERVICE, name)?;
    Ok(entry.delete_credential()?)
}

pub fn config_path() -> anyhow::Result<PathBuf> {
    let home = home_dir()?;
    Ok(home.join(".config").join("seacloud").join("config.yml"))
}

fn home_dir() -> anyhow::Result<PathBuf> {
    if let Some(home) = env::var_os("HOME") {
        return Ok(PathBuf::from(home));
    }
    if cfg!(windows) {
        if let Some(home) = env::var_os("USERPROFILE") {
            return Ok(PathBuf::from(home));
        }
    }
    dirs::home_dir().ok_or_else(|| anyhow::anyhow!("failed to resolve home directory"))
}

pub fn load() -> anyhow::Result<Config> {
    let mut cfg = load_stored()?;
    if let Some((token, source)) = managed_token_from_env() {
        cfg.auth_token = token.clone();
        cfg.refresh_token.clear();
        cfg.api_key = token;
        cfg.managed = true;
        cfg.runtime = runtime_from_env();
        if cfg.runtime.is_empty() {
            cfg.runtime = RUNTIME_FOLKOS.to_string();
        }
        cfg.credential_source = source;
    }
    Ok(cfg)
}

pub fn load_stored() -> anyhow::Result<Config> {
    let (auth_token, refresh_token, api_key) = token_store().load_tokens()?;
    Ok(Config {
        auth_token,
        refresh_token,
        api_key,
        ..Config::default()
    })
}

pub fn save(cfg: &Config) -> anyhow::Result<()> {
    token_store().save_tokens(&cfg.auth_token, &cfg.refresh_token, &cfg.api_key)
}

pub fn clear() -> anyhow::Result<()> {
    token_store().clear_tokens()
}

pub fn exec_token_from_env() -> String {
    managed_token_from_env()
        .map(|(token, _)| token)
        .unwrap_or_default()
}

pub fn folkos_runtime_headers() -> BTreeMap<String, String> {
    let mut headers = BTreeMap::new();
    if env_value(ENV_FOLKOS_EXEC_TOKEN).is_some() {
        headers.insert("X-Folkos-Token-Kind".to_string(), "execution".to_string());
    }
    add_header(&mut headers, "X-Folkos-Session-ID", ENV_FOLKOS_SESSION_ID);
    add_header(&mut headers, "X-Folkos-Turn-ID", ENV_FOLKOS_TURN_ID);
    add_header(&mut headers, "X-Folkos-Message-ID", ENV_FOLKOS_MESSAGE_ID);
    add_header(&mut headers, "X-Folkos-Agent-UUID", ENV_FOLKOS_AGENT_UUID);
    add_header(
        &mut headers,
        "X-Folkos-Workspace-ID",
        ENV_FOLKOS_WORKSPACE_ID,
    );
    add_header(
        &mut headers,
        "X-Folkos-Agent-Temp-ID",
        ENV_FOLKOS_AGENT_TEMP_ID,
    );
    add_header(&mut headers, "X-Folkos-Sandbox-ID", ENV_FOLKOS_SANDBOX_ID);
    headers
}

fn add_header(headers: &mut BTreeMap<String, String>, header: &str, env_name: &str) {
    if let Some(value) = env_value(env_name) {
        headers.insert(header.to_string(), value);
    }
}

pub fn runtime_from_env() -> String {
    env::var(ENV_SEACLOUD_RUNTIME)
        .unwrap_or_default()
        .trim()
        .to_ascii_lowercase()
}

pub fn use_folkos_proxy() -> bool {
    runtime_from_env() == RUNTIME_FOLKOS || managed_token_from_env().is_some()
}

pub fn folkos_proxy_base_url() -> String {
    if !use_folkos_proxy() {
        return String::new();
    }
    if let Some(gateway_url) = folkos_proxy_base_url_from_gateway_env() {
        return gateway_url;
    }
    normalize_absolute_url(default_folkos_proxy_base_url()).unwrap_or_default()
}

pub fn rewrite_url_through_folkos_proxy(raw: &str) -> String {
    let proxy_base = folkos_proxy_base_url();
    if proxy_base.is_empty() {
        return raw.to_string();
    }

    let Ok(target) = Url::parse(raw) else {
        return raw.to_string();
    };
    let Some(host) = target.host_str().map(str::to_ascii_lowercase) else {
        return raw.to_string();
    };
    if host != "vtrix.ai" && !host.ends_with(".vtrix.ai") {
        return raw.to_string();
    }

    let Ok(mut proxy_url) = Url::parse(&proxy_base) else {
        return raw.to_string();
    };
    let base_path = proxy_url.path().trim_end_matches('/');
    let target_path = target.path().trim_start_matches('/');
    let path = if base_path.is_empty() {
        format!("/{target_path}")
    } else {
        format!("{base_path}/{target_path}")
    };
    proxy_url.set_path(&path);
    proxy_url.set_query(target.query());
    proxy_url.set_fragment(target.fragment());
    proxy_url.to_string()
}

fn legacy_file_tokens() -> Option<[String; 3]> {
    let tokens = read_file_tokens()?;
    if tokens.auth_token.is_empty() {
        return None;
    }
    Some([tokens.auth_token, tokens.refresh_token, tokens.api_key])
}

fn read_file_tokens() -> Option<FileTokens> {
    let path = config_path().ok()?;
    let data = fs::read_to_string(path).ok()?;
    serde_yaml::from_str(&data).ok()
}

fn clear_legacy_file_tokens() -> anyhow::Result<()> {
    let path = config_path()?;
    let data = serde_yaml::to_string(&FileConfig::default())?;
    write_private_file(&path, data.as_bytes())?;
    Ok(())
}

fn normalize_absolute_url(raw: impl AsRef<str>) -> Option<String> {
    let raw = raw.as_ref().trim();
    if raw.is_empty() {
        return None;
    }
    let mut url = Url::parse(raw).ok()?;
    if url.host_str().is_none() || url.scheme().is_empty() {
        return None;
    }
    url.set_query(None);
    url.set_fragment(None);
    let path = url.path().trim_end_matches('/').to_string();
    url.set_path(&path);
    Some(url.to_string().trim_end_matches('/').to_string())
}

fn folkos_proxy_base_url_from_gateway_env() -> Option<String> {
    let gateway_url = normalize_absolute_url(env::var(ENV_GATEWAY_URL).unwrap_or_default())?;
    let mut url = Url::parse(&gateway_url).ok()?;
    let path = url.path().trim_end_matches('/').to_string();
    if path.is_empty() {
        url.set_path("/folkos-proxy");
    } else if !path.ends_with("/folkos-proxy") {
        url.set_path(&format!("{path}/folkos-proxy"));
    } else {
        url.set_path(&path);
    }
    Some(url.to_string().trim_end_matches('/').to_string())
}

fn managed_token_from_env() -> Option<(String, String)> {
    if let Some(token) = env_value(ENV_FOLKOS_EXEC_TOKEN) {
        return Some((token, ENV_FOLKOS_EXEC_TOKEN.to_string()));
    }
    if let Some(token) = env_value(ENV_FOLKOS_TOKEN) {
        return Some((token, ENV_FOLKOS_TOKEN.to_string()));
    }
    None
}

fn env_value(name: &str) -> Option<String> {
    let value = env::var(name).unwrap_or_default().trim().to_string();
    if value.is_empty() {
        None
    } else {
        Some(value)
    }
}

fn default_folkos_proxy_base_url() -> String {
    env::var("SEACLOUD_DEFAULT_FOLKOS_PROXY_URL")
        .ok()
        .or_else(|| option_env!("SEACLOUD_FOLKOS_PROXY_URL").map(str::to_string))
        .unwrap_or_default()
}

#[cfg(unix)]
fn write_private_file(path: &std::path::Path, data: &[u8]) -> anyhow::Result<()> {
    use std::os::unix::fs::OpenOptionsExt;

    if let Some(parent) = path.parent() {
        fs::create_dir_all(parent)?;
    }
    let mut options = fs::OpenOptions::new();
    options.create(true).truncate(true).write(true).mode(0o600);
    std::io::Write::write_all(&mut options.open(path)?, data)?;
    Ok(())
}

#[cfg(not(unix))]
fn write_private_file(path: &std::path::Path, data: &[u8]) -> anyhow::Result<()> {
    if let Some(parent) = path.parent() {
        fs::create_dir_all(parent)?;
    }
    fs::write(path, data)?;
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;
    use serial_test::serial;
    use tempfile::TempDir;

    fn isolated_home() -> TempDir {
        let dir = TempDir::new().unwrap();
        env::set_var("HOME", dir.path());
        env::set_var("SEACLOUD_NO_KEYCHAIN", "1");
        dir
    }

    fn clear_env() {
        for key in [
            ENV_FOLKOS_EXEC_TOKEN,
            ENV_FOLKOS_TOKEN,
            ENV_SEACLOUD_RUNTIME,
            ENV_GATEWAY_URL,
            "SEACLOUD_DEFAULT_FOLKOS_PROXY_URL",
        ] {
            env::remove_var(key);
        }
    }

    fn write_config_file(home: &std::path::Path, content: &str) {
        let path = home.join(".config").join("seacloud").join("config.yml");
        fs::create_dir_all(path.parent().unwrap()).unwrap();
        fs::write(path, content).unwrap();
    }

    #[test]
    #[serial]
    fn load_stored_without_managed_env() {
        clear_env();
        let home = isolated_home();
        write_config_file(
            home.path(),
            "auth_token: stored-auth\nrefresh_token: stored-refresh\napi_key: stored-key\n",
        );

        let cfg = load().unwrap();
        assert!(!cfg.managed);
        assert_eq!(cfg.auth_token, "stored-auth");
        assert_eq!(cfg.refresh_token, "stored-refresh");
        assert_eq!(cfg.api_key, "stored-key");
    }

    #[test]
    #[serial]
    fn load_managed_exec_token_overrides_stored_credentials() {
        clear_env();
        let home = isolated_home();
        env::set_var(ENV_FOLKOS_EXEC_TOKEN, "exec-token");
        env::set_var(ENV_SEACLOUD_RUNTIME, RUNTIME_FOLKOS);
        write_config_file(
            home.path(),
            "auth_token: stored-auth\nrefresh_token: stored-refresh\napi_key: stored-key\n",
        );

        let cfg = load().unwrap();
        assert!(cfg.managed);
        assert_eq!(cfg.runtime, RUNTIME_FOLKOS);
        assert_eq!(cfg.credential_source, ENV_FOLKOS_EXEC_TOKEN);
        assert_eq!(cfg.auth_token, "exec-token");
        assert_eq!(cfg.api_key, "exec-token");
        assert!(cfg.refresh_token.is_empty());

        let stored = load_stored().unwrap();
        assert_eq!(stored.auth_token, "stored-auth");
        assert_eq!(stored.refresh_token, "stored-refresh");
        assert_eq!(stored.api_key, "stored-key");
    }

    #[test]
    #[serial]
    fn folkos_proxy_base_url_prefers_gateway_url() {
        clear_env();
        env::set_var(ENV_FOLKOS_EXEC_TOKEN, "exec-token");
        env::set_var(ENV_SEACLOUD_RUNTIME, RUNTIME_FOLKOS);
        env::set_var(ENV_GATEWAY_URL, "https://gateway.example.com/");

        assert_eq!(
            folkos_proxy_base_url(),
            "https://gateway.example.com/folkos-proxy"
        );
    }

    #[test]
    #[serial]
    fn rewrite_url_through_folkos_proxy_rewrites_only_vtrix_urls() {
        clear_env();
        env::set_var(ENV_SEACLOUD_RUNTIME, RUNTIME_FOLKOS);
        env::set_var(
            "SEACLOUD_DEFAULT_FOLKOS_PROXY_URL",
            "https://gateway.example.com/folkos-proxy",
        );

        assert_eq!(
            rewrite_url_through_folkos_proxy("https://cloud.vtrix.ai/model/v1/generation?debug=1"),
            "https://gateway.example.com/folkos-proxy/model/v1/generation?debug=1"
        );
        assert_eq!(
            rewrite_url_through_folkos_proxy("https://api.openai.com/v1/responses"),
            "https://api.openai.com/v1/responses"
        );
    }

    #[test]
    #[serial]
    fn file_store_save_and_clear_round_trip() {
        clear_env();
        let home = isolated_home();
        let cfg = Config {
            auth_token: "auth".to_string(),
            refresh_token: "refresh".to_string(),
            api_key: "key".to_string(),
            ..Config::default()
        };

        save(&cfg).unwrap();
        let stored = load_stored().unwrap();
        assert_eq!(stored.auth_token, "auth");
        assert_eq!(stored.refresh_token, "refresh");
        assert_eq!(stored.api_key, "key");

        clear().unwrap();
        assert!(!home.path().join(".config/seacloud/config.yml").exists());
    }

    #[test]
    #[serial]
    fn runtime_headers_include_managed_context() {
        clear_env();
        env::set_var(ENV_FOLKOS_EXEC_TOKEN, "exec");
        env::set_var(ENV_FOLKOS_SESSION_ID, "session");
        env::set_var(ENV_FOLKOS_TURN_ID, "turn");
        env::set_var(ENV_FOLKOS_MESSAGE_ID, "message");
        env::set_var(ENV_FOLKOS_AGENT_UUID, "agent");
        env::set_var(ENV_FOLKOS_WORKSPACE_ID, "workspace");
        env::set_var(ENV_FOLKOS_AGENT_TEMP_ID, "temp");
        env::set_var(ENV_FOLKOS_SANDBOX_ID, "sandbox");

        let headers = folkos_runtime_headers();
        assert_eq!(headers["X-Folkos-Token-Kind"], "execution");
        assert_eq!(headers["X-Folkos-Session-ID"], "session");
        assert_eq!(headers["X-Folkos-Turn-ID"], "turn");
        assert_eq!(headers["X-Folkos-Message-ID"], "message");
        assert_eq!(headers["X-Folkos-Agent-UUID"], "agent");
        assert_eq!(headers["X-Folkos-Workspace-ID"], "workspace");
        assert_eq!(headers["X-Folkos-Agent-Temp-ID"], "temp");
        assert_eq!(headers["X-Folkos-Sandbox-ID"], "sandbox");
        assert_eq!(exec_token_from_env(), "exec");
    }

    #[test]
    #[serial]
    fn folkos_proxy_base_url_handles_paths_and_disabled_runtime() {
        clear_env();
        env::set_var(ENV_FOLKOS_TOKEN, "managed");
        env::set_var(ENV_GATEWAY_URL, "https://gateway.example.com/base");
        assert_eq!(
            folkos_proxy_base_url(),
            "https://gateway.example.com/base/folkos-proxy"
        );

        clear_env();
        env::set_var(ENV_GATEWAY_URL, "https://gateway.example.com");
        assert!(!use_folkos_proxy());
        assert_eq!(folkos_proxy_base_url(), "");
    }

    #[test]
    #[serial]
    fn rewrite_url_handles_invalid_and_existing_proxy_path() {
        clear_env();
        env::set_var(ENV_SEACLOUD_RUNTIME, RUNTIME_FOLKOS);
        env::set_var(ENV_GATEWAY_URL, "https://gateway.example.com/folkos-proxy");

        assert_eq!(rewrite_url_through_folkos_proxy("not a url"), "not a url");
        assert_eq!(
            rewrite_url_through_folkos_proxy("https://vtrix.ai/path#frag"),
            "https://gateway.example.com/folkos-proxy/path#frag"
        );
    }
}
