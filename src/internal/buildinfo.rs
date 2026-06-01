pub const NAME: &str = "seacloud";
pub const VERSION: &str = match option_env!("SEACLOUD_BUILD_VERSION") {
    Some(version) => version,
    None => "dev",
};

pub fn user_agent() -> String {
    format!("{NAME}-cli/{VERSION}")
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn user_agent_contains_name_and_version() {
        assert_eq!(NAME, "seacloud");
        assert!(user_agent().starts_with("seacloud-cli/"));
    }
}
