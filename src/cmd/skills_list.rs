use super::root::Context;
use crate::internal::skillhub;
use clap::Args;

#[derive(Debug, Args)]
pub struct SkillsListArgs {
    #[arg(short = 'c', long, default_value = "", help = "Filter by category")]
    pub(crate) category: String,
    #[arg(
        short = 's',
        long,
        default_value = "",
        help = "Sort by (stars, downloads, updated)"
    )]
    pub(crate) sort: String,
}

pub fn handle(args: SkillsListArgs, _ctx: Context) -> anyhow::Result<()> {
    let client = skillhub::Client::new();
    client.list(&args.category, &args.sort)
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::internal::test_support::{TestResponse, TestServer};
    use serial_test::serial;

    #[test]
    #[serial]
    fn list_command_calls_skillhub() {
        let server = TestServer::new(|req| {
            assert_eq!(req.path, "/search?q=&limit=20&category=image");
            TestResponse::json(
                200,
                r#"{"results":[{"slug":"cat","displayName":"Cat","description":"desc"}]}"#,
            )
        });
        std::env::set_var("SEACLOUD_SKILLHUB_URL", server.url());
        handle(
            SkillsListArgs {
                category: "image".to_string(),
                sort: "updated".to_string(),
            },
            Context { dry_run: false },
        )
        .unwrap();
    }
}
