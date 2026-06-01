use super::root::Context;
use crate::internal::skillhub;
use clap::Args;

#[derive(Debug, Args)]
pub struct SkillsAddArgs {
    pub(crate) slug: String,
    #[arg(
        short = 'v',
        long,
        default_value = "",
        help = "Specific version to install (default: latest)"
    )]
    pub(crate) version: String,
    #[arg(short = 'g', long, help = "Install globally (auto-detects agent)")]
    pub(crate) global: bool,
    #[arg(short = 'y', long = "yes", help = "Skip confirmation prompts")]
    pub(crate) yes: bool,
}

pub fn handle(args: SkillsAddArgs, ctx: Context) -> anyhow::Result<()> {
    if ctx.dry_run {
        return dry_run_skill_add(&args.slug, &args.version, args.global);
    }
    let client = skillhub::Client::new();
    client.add(&args.slug, &args.version, args.global, args.yes)
}

fn dry_run_skill_add(slug: &str, version: &str, _global: bool) -> anyhow::Result<()> {
    eprintln!("[dry-run] Would install skill: {slug}");
    if !version.is_empty() {
        eprintln!("[dry-run]   Version: {version}");
    } else {
        eprintln!("[dry-run]   Version: latest");
    }

    let home = std::env::var("HOME").unwrap_or_default();
    let global_repo = format!("{home}/.agents/skills/{slug}");
    eprintln!("[dry-run]   Target: {global_repo}");

    let agents = skillhub::detect_all_installed_agents();
    if !agents.is_empty() {
        let names: Vec<_> = agents
            .iter()
            .map(|agent| agent.display_name.as_str())
            .collect();
        eprintln!("[dry-run]   Detected agents: {}", names.join(", "));
        let link_action = if cfg!(windows) { "copies" } else { "symlinks" };
        eprintln!(
            "[dry-run]   Would create {} to {} agent(s)",
            link_action,
            agents.len()
        );
    }

    eprintln!("[dry-run] No changes made. Remove --dry-run to install.");
    Ok(())
}
