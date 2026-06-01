use serde::Deserialize;
use std::collections::HashMap;
use std::sync::OnceLock;

#[derive(Debug, Clone, Deserialize)]
struct AliasPrefixRule {
    display_prefix: String,
    backend_prefix: String,
}

#[derive(Debug, Clone, Deserialize)]
struct AliasConfig {
    #[serde(default)]
    exact: HashMap<String, String>,
    #[serde(default)]
    prefixes: Vec<AliasPrefixRule>,
}

static ALIAS_CONFIG: OnceLock<AliasConfig> = OnceLock::new();
static REVERSE_EXPLICIT_ALIASES: OnceLock<HashMap<String, String>> = OnceLock::new();

fn alias_config() -> &'static AliasConfig {
    ALIAS_CONFIG.get_or_init(|| {
        let mut cfg: AliasConfig = serde_yaml::from_str(include_str!("model_aliases.yaml"))
            .expect("load model alias config");
        for rule in &mut cfg.prefixes {
            rule.display_prefix = rule.display_prefix.trim().to_string();
            rule.backend_prefix = rule.backend_prefix.trim().to_string();
            if rule.display_prefix.is_empty() || rule.backend_prefix.is_empty() {
                panic!("load model alias config: empty display_prefix/backend_prefix");
            }
        }
        cfg
    })
}

fn reverse_explicit_aliases() -> &'static HashMap<String, String> {
    REVERSE_EXPLICIT_ALIASES.get_or_init(|| {
        alias_config()
            .exact
            .iter()
            .map(|(display, backend)| (backend.clone(), display.clone()))
            .collect()
    })
}

pub fn resolve_model_id(model_id: &str) -> String {
    let model_id = model_id.trim();
    if model_id.is_empty() {
        return String::new();
    }

    if let Some(resolved) = alias_config().exact.get(model_id) {
        return resolved.clone();
    }

    for rule in &alias_config().prefixes {
        if let Some(rest) = model_id.strip_prefix(&rule.display_prefix) {
            return format!("{}{}", rule.backend_prefix, rest);
        }
    }

    model_id.to_string()
}

pub fn display_model_id(model_id: &str) -> String {
    let model_id = model_id.trim();
    if model_id.is_empty() {
        return String::new();
    }

    if let Some(display) = reverse_explicit_aliases().get(model_id) {
        return display.clone();
    }

    for rule in &alias_config().prefixes {
        if let Some(rest) = model_id.strip_prefix(&rule.backend_prefix) {
            return format!("{}{}", rule.display_prefix, rest);
        }
    }

    model_id.to_string()
}

pub fn preferred_model_id(requested_model_id: &str, backend_model_id: &str) -> String {
    let requested_model_id = requested_model_id.trim();
    let backend_model_id = backend_model_id.trim();

    if requested_model_id.is_empty() {
        return display_model_id(backend_model_id);
    }
    if backend_model_id.is_empty() || requested_model_id == backend_model_id {
        return requested_model_id.to_string();
    }
    if resolve_model_id(requested_model_id) == backend_model_id {
        return requested_model_id.to_string();
    }
    display_model_id(backend_model_id)
}

pub fn rewrite_model_id_text(text: &str, backend_model_id: &str, display_model_id: &str) -> String {
    let backend_model_id = backend_model_id.trim();
    let display_model_id = display_model_id.trim();
    if text.is_empty()
        || backend_model_id.is_empty()
        || display_model_id.is_empty()
        || backend_model_id == display_model_id
    {
        return text.to_string();
    }
    text.replace(backend_model_id, display_model_id)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn embedded_alias_config_loaded() {
        assert_eq!(
            alias_config().exact.get("kling_image_o1").unwrap(),
            "kirin_image_omni"
        );
        assert_eq!(alias_config().prefixes.len(), 3);
    }

    #[test]
    fn resolve_and_display_model_id() {
        let cases = [
            ("kling_duration_extension", "kirin_duration_extension"),
            ("kling_image_o1", "kirin_image_omni"),
            ("kling_v2_6_i2v", "kirin_v2_6_i2v"),
            ("seedance_2_0", "spark_dance_v2_0"),
            ("seedream_4_5", "spark_dream_4_5"),
            ("gpt-image-2", "gpt-image-2"),
        ];
        for (display, backend) in cases {
            assert_eq!(resolve_model_id(display), backend);
            assert_eq!(display_model_id(backend), display);
        }
    }

    #[test]
    fn preferred_and_rewrite_model_id() {
        assert_eq!(
            preferred_model_id("seedance_2_0", "spark_dance_v2_0"),
            "seedance_2_0"
        );
        assert_eq!(
            rewrite_model_id_text(
                "model=kirin_v3_t2v endpoint=kirin_v3_t2v",
                "kirin_v3_t2v",
                "kling_v3_t2v"
            ),
            "model=kling_v3_t2v endpoint=kling_v3_t2v"
        );
    }
}
