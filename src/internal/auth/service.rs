use super::client::{Client, DeviceCodeRequest, MeResponse, TokenRequest};
use crate::internal::clierrors::CliError;
use base64::engine::general_purpose::URL_SAFE_NO_PAD;
use base64::Engine;
use ring::rand::SystemRandom;
use ring::signature::{Ed25519KeyPair, KeyPair};
use sha2::{Digest, Sha256};
use std::thread;
use std::time::{Duration, Instant, SystemTime, UNIX_EPOCH};
use url::Url;
use uuid::Uuid;

const CLIENT_ID: &str = "seacloud-cli";

fn generate_key_pair() -> anyhow::Result<(String, Ed25519KeyPair)> {
    let rng = SystemRandom::new();
    let pkcs8 = Ed25519KeyPair::generate_pkcs8(&rng)
        .map_err(|_| anyhow::anyhow!("failed to generate Ed25519 key pair"))?;
    let key_pair = Ed25519KeyPair::from_pkcs8(pkcs8.as_ref())
        .map_err(|_| anyhow::anyhow!("failed to parse Ed25519 key pair"))?;
    let public_key = URL_SAFE_NO_PAD.encode(key_pair.public_key().as_ref());
    Ok((public_key, key_pair))
}

fn build_proof(
    key_pair: &Ed25519KeyPair,
    device_code: &str,
    timestamp: i64,
    nonce: &str,
) -> String {
    let msg = format!("{device_code}:{timestamp}:{nonce}");
    let hash = Sha256::digest(msg.as_bytes());
    let sig = key_pair.sign(&hash);
    URL_SAFE_NO_PAD.encode(sig.as_ref())
}

pub fn login(
    open_browser: impl Fn(&str) -> anyhow::Result<()>,
) -> anyhow::Result<(String, String, String)> {
    let (pub_key, key_pair) =
        generate_key_pair().map_err(|err| anyhow::anyhow!("failed to generate key pair: {err}"))?;

    let client = Client::new("");
    let dc = client
        .request_device_code(DeviceCodeRequest {
            client_id: CLIENT_ID.to_string(),
            client_public_key: pub_key,
        })
        .map_err(|err| anyhow::anyhow!("failed to connect to SeaCloud: {err}"))?;

    let auth_url = build_verification_url(&dc.verification_uri, &dc.user_code);
    println!("\nURL:  {auth_url}");
    println!("Code: {}\n", dc.user_code);
    if open_browser(&auth_url).is_err() {
        println!("(Could not open browser automatically. Please visit the URL above.)");
    } else {
        println!("Opened your browser automatically. If the page does not load, visit the URL above manually.");
    }
    println!("Waiting for authorization...");

    let mut interval = Duration::from_secs(dc.interval.max(0) as u64);
    if interval < Duration::from_secs(1) {
        interval = Duration::from_secs(5);
    }
    let deadline = Instant::now() + Duration::from_secs(dc.expires_in.max(0) as u64);

    while Instant::now() < deadline {
        thread::sleep(interval);
        let timestamp = SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .unwrap_or_default()
            .as_secs() as i64;
        let nonce = Uuid::new_v4().to_string();
        let proof = build_proof(&key_pair, &dc.device_code, timestamp, &nonce);
        let result = match client.poll_token(TokenRequest {
            device_code: dc.device_code.clone(),
            timestamp: timestamp.to_string(),
            nonce,
            proof,
        }) {
            Ok(result) => result,
            Err(_) => continue,
        };

        match result.status.as_str() {
            "authorized" | "" => {
                if !result.access_token.is_empty() {
                    return Ok((result.access_token, result.refresh_token, result.api_key));
                }
            }
            "expired" => {
                return Err(CliError::with_hint(
                    "authorization code expired",
                    "Run: seacloud auth login",
                )
                .into());
            }
            "pending" => {}
            _ => {}
        }
    }

    Err(CliError::with_hint("authorization timed out", "Run: seacloud auth login").into())
}

pub fn verify_token(token: &str) -> anyhow::Result<MeResponse> {
    Client::new(token).me()
}

fn build_verification_url(verification_uri: &str, user_code: &str) -> String {
    if verification_uri.is_empty() || user_code.is_empty() {
        return verification_uri.to_string();
    }
    let Ok(mut url) = Url::parse(verification_uri) else {
        return verification_uri.to_string();
    };
    url.query_pairs_mut().append_pair("code", user_code);
    url.to_string()
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn verification_url_adds_code() {
        assert_eq!(
            build_verification_url("https://example.com/login", "ABCD"),
            "https://example.com/login?code=ABCD"
        );
    }

    #[test]
    fn proof_generation_and_invalid_verification_url_are_stable() {
        let (pub_key, key_pair) = generate_key_pair().unwrap();
        assert!(!pub_key.is_empty());
        let proof = build_proof(&key_pair, "device", 1, "nonce");
        assert!(!proof.is_empty());
        assert_eq!(build_verification_url(":", "ABCD"), ":");
        assert_eq!(build_verification_url("", "ABCD"), "");
    }
}
