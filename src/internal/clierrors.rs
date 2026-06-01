use std::fmt;

#[derive(Debug)]
pub struct CliError {
    pub message: String,
    pub hint: Option<String>,
}

impl CliError {
    pub fn new(message: impl Into<String>) -> Self {
        Self {
            message: message.into(),
            hint: None,
        }
    }

    pub fn with_hint(message: impl Into<String>, hint: impl Into<String>) -> Self {
        Self {
            message: message.into(),
            hint: Some(hint.into()),
        }
    }
}

impl fmt::Display for CliError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match &self.hint {
            Some(hint) if !hint.is_empty() => write!(f, "{}\n  Hint: {}", self.message, hint),
            _ => write!(f, "{}", self.message),
        }
    }
}

impl std::error::Error for CliError {}

pub fn err_token_expired() -> CliError {
    CliError::with_hint("session expired", "Run: seacloud auth login")
}

pub fn err_token_invalid() -> CliError {
    CliError::with_hint("invalid token", "Run: seacloud auth login")
}

pub fn err_token_verification(err: impl fmt::Display) -> CliError {
    CliError::with_hint(
        format!("token verification failed: {err}"),
        "Run: seacloud auth login",
    )
}

pub fn err_save_config(err: impl fmt::Display) -> CliError {
    CliError::with_hint(
        format!("failed to save config: {err}"),
        "Check write permissions for ~/.config/seacloud/",
    )
}

pub fn err_logout(err: impl fmt::Display) -> CliError {
    CliError::with_hint(
        format!("failed to clear credentials: {err}"),
        "Try deleting ~/.config/seacloud/config.yml manually",
    )
}

pub fn err_network(err: impl fmt::Display) -> CliError {
    CliError::with_hint(
        format!("network error: {err}"),
        "Check your network connection and that the SeaCloud API is reachable",
    )
}

pub fn err_network_timeout(err: impl fmt::Display) -> CliError {
    CliError::with_hint(
        format!("request timed out: {err}"),
        "Check your network connection or try again",
    )
}

pub fn err_model_not_found(id: &str) -> CliError {
    CliError::with_hint(
        format!("model {id:?} not found"),
        "Run: seacloud models list to see available models",
    )
}

pub fn err_fetch_models(err: impl fmt::Display) -> CliError {
    CliError::with_hint(
        format!("failed to fetch models: {err}"),
        "Check your network connection and try again",
    )
}

pub fn err_fetch_model_spec(id: &str, err: impl fmt::Display) -> CliError {
    CliError::with_hint(
        format!("failed to fetch spec for {id:?}: {err}"),
        "Run: seacloud models list to see available models",
    )
}

pub fn err_no_api_key() -> CliError {
    CliError::with_hint(
        "API key not set",
        "Run: seacloud auth login to obtain an API key, or inject FOLKOS_EXEC_TOKEN in managed runtimes",
    )
}

pub fn err_managed_credentials_override() -> CliError {
    CliError::with_hint(
        "credentials are managed by the runtime",
        "Unset FOLKOS_EXEC_TOKEN to manage credentials locally with seacloud auth login or auth set-key",
    )
}

pub fn err_invalid_param(model_id: &str, name: &str, reason: impl fmt::Display) -> CliError {
    CliError::with_hint(
        format!("invalid value for parameter {name:?}: {reason}"),
        format!("Run: seacloud models spec {model_id} to see allowed values"),
    )
}

pub fn err_missing_param(model_id: &str, name: &str) -> CliError {
    CliError::with_hint(
        format!("missing required parameter: {name:?}"),
        format!("Run: seacloud models spec {model_id} to see required parameters"),
    )
}

pub fn err_submit_failed(err: impl fmt::Display) -> CliError {
    CliError::with_hint(
        format!("generation request failed: {err}"),
        "Check your API key with: seacloud auth status",
    )
}

pub fn err_task_failed(task_id: &str, reason: &str) -> CliError {
    CliError::new(format!("task {task_id} failed: {reason}"))
}

pub fn err_task_timeout(task_id: &str) -> CliError {
    CliError::with_hint(
        format!("task {task_id} timed out waiting for result"),
        format!("Run: seacloud task status {task_id} to check later"),
    )
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn cli_error_formats_hint() {
        assert_eq!(
            CliError::with_hint("message", "do thing").to_string(),
            "message\n  Hint: do thing"
        );
        assert_eq!(CliError::new("message").to_string(), "message");
    }

    #[test]
    fn constructors_keep_expected_messages() {
        let errors = [
            err_token_expired().to_string(),
            err_token_invalid().to_string(),
            err_token_verification("bad").to_string(),
            err_save_config("denied").to_string(),
            err_logout("denied").to_string(),
            err_network("offline").to_string(),
            err_network_timeout("slow").to_string(),
            err_model_not_found("m").to_string(),
            err_fetch_models("bad").to_string(),
            err_fetch_model_spec("m", "bad").to_string(),
            err_no_api_key().to_string(),
            err_managed_credentials_override().to_string(),
            err_invalid_param("m", "p", "bad").to_string(),
            err_missing_param("m", "p").to_string(),
            err_submit_failed("bad").to_string(),
            err_task_failed("t", "bad").to_string(),
            err_task_timeout("t").to_string(),
        ];

        assert!(errors.iter().any(|err| err.contains("session expired")));
        assert!(errors.iter().any(|err| err.contains("API key not set")));
        assert!(errors.iter().any(|err| err.contains("task t failed")));
        assert!(errors.iter().all(|err| !err.is_empty()));
    }
}
