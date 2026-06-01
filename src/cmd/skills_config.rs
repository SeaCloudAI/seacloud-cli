use super::root::Context;
use crate::internal::skillhub;
use clap::Args;

#[derive(Debug, Args)]
pub struct SkillsConfigArgs {
    #[arg(long = "set-url", default_value = "", help = "Set API base URL")]
    set_url: String,
    #[arg(long, help = "Show current configuration")]
    show: bool,
}

pub fn handle(args: SkillsConfigArgs, _ctx: Context) -> anyhow::Result<()> {
    let client = skillhub::Client::new();
    client.config(&args.set_url, args.show)
}

#[cfg(test)]
mod tests {
    use super::*;
    use serial_test::serial;
    use tempfile::TempDir;

    #[test]
    #[serial]
    fn config_command_sets_and_shows_url() {
        let home = TempDir::new().unwrap();
        std::env::set_var("HOME", home.path());
        handle(
            SkillsConfigArgs {
                set_url: "https://skillhub.example.com/api".to_string(),
                show: false,
            },
            Context { dry_run: false },
        )
        .unwrap();
        handle(
            SkillsConfigArgs {
                set_url: String::new(),
                show: true,
            },
            Context { dry_run: false },
        )
        .unwrap();
    }
}
