use super::root::Context;
use crate::internal::skillhub;
use clap::Args;

#[derive(Debug, Args)]
pub struct SkillsFindArgs {
    pub(crate) query: Option<String>,
    #[arg(short = 'c', long, default_value = "", help = "Filter by category")]
    pub(crate) category: String,
    #[arg(short = 'i', long, help = "Interactive mode (browse by category)")]
    pub(crate) interactive: bool,
    #[arg(long, default_value = "", help = "Page cursor for pagination")]
    pub(crate) cursor: String,
}

pub fn handle(args: SkillsFindArgs, _ctx: Context) -> anyhow::Result<()> {
    let client = skillhub::Client::new();
    client.find(
        args.query.as_deref().unwrap_or_default(),
        &args.category,
        args.interactive,
        &args.cursor,
    )
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::internal::test_support::{TestResponse, TestServer};
    use serial_test::serial;

    #[test]
    #[serial]
    fn find_command_calls_skillhub() {
        let server = TestServer::new(|req| {
            assert_eq!(req.path, "/search?q=cat&limit=20&category=image&cursor=c1");
            TestResponse::json(
                200,
                r#"{"results":[{"slug":"cat","displayName":"Cat","description":"desc"}]}"#,
            )
        });
        std::env::set_var("SEACLOUD_SKILLHUB_URL", server.url());
        handle(
            SkillsFindArgs {
                query: Some("cat".to_string()),
                category: "image".to_string(),
                interactive: false,
                cursor: "c1".to_string(),
            },
            Context { dry_run: false },
        )
        .unwrap();
    }
}
