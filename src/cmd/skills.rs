use super::root::Context;
use super::{skills_add, skills_config, skills_find, skills_list};
use clap::{Args, Subcommand};

#[derive(Debug, Args)]
pub struct SkillsCommand {
    #[command(subcommand)]
    command: SkillsSubcommand,
}

#[derive(Debug, Subcommand)]
enum SkillsSubcommand {
    #[command(about = "Install a skill")]
    Add(skills_add::SkillsAddArgs),
    #[command(about = "Configure SkillHub CLI")]
    Config(skills_config::SkillsConfigArgs),
    #[command(about = "Search for skills")]
    Find(skills_find::SkillsFindArgs),
    #[command(about = "List skills")]
    List(skills_list::SkillsListArgs),
}

pub fn handle(cmd: SkillsCommand, ctx: Context) -> anyhow::Result<()> {
    match cmd.command {
        SkillsSubcommand::Add(args) => skills_add::handle(args, ctx),
        SkillsSubcommand::Config(args) => skills_config::handle(args, ctx),
        SkillsSubcommand::Find(args) => skills_find::handle(args, ctx),
        SkillsSubcommand::List(args) => skills_list::handle(args, ctx),
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::internal::test_support::{TestResponse, TestServer};
    use serial_test::serial;

    #[test]
    #[serial]
    fn skills_dispatches_all_subcommands() {
        let server = TestServer::new(|req| {
            if req.path.starts_with("/search") {
                return TestResponse::json(200, r#"{"results":[]}"#);
            }
            TestResponse::json(500, "{}")
        });
        std::env::set_var("SEACLOUD_SKILLHUB_URL", server.url());

        handle(
            SkillsCommand {
                command: SkillsSubcommand::Find(skills_find::SkillsFindArgs {
                    query: None,
                    category: String::new(),
                    interactive: false,
                    cursor: String::new(),
                }),
            },
            Context { dry_run: false },
        )
        .unwrap();
        handle(
            SkillsCommand {
                command: SkillsSubcommand::List(skills_list::SkillsListArgs {
                    category: String::new(),
                    sort: String::new(),
                }),
            },
            Context { dry_run: false },
        )
        .unwrap();
        handle(
            SkillsCommand {
                command: SkillsSubcommand::Add(skills_add::SkillsAddArgs {
                    slug: "cat".to_string(),
                    version: String::new(),
                    global: false,
                    yes: true,
                }),
            },
            Context { dry_run: true },
        )
        .unwrap();
    }
}
