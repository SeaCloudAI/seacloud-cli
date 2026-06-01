use super::{auth, images, models, run, skills, task, version};
use crate::internal::buildinfo;
use clap::{CommandFactory, Parser, Subcommand};

#[derive(Debug, Parser)]
#[command(
    name = "seacloud",
    about = "SeaCloud CLI - Access multimodal AI with a single API Key",
    long_about = "SeaCloud CLI lets you manage your account, browse models, and call multimodal AI services via API Key.",
    disable_version_flag = true
)]
pub struct Cli {
    #[arg(
        long,
        global = true,
        help = "Print what would be executed without making any changes"
    )]
    pub dry_run: bool,

    #[arg(short = 'v', long = "version", help = "version for seacloud")]
    pub version: bool,

    #[command(subcommand)]
    pub command: Option<Commands>,
}

#[derive(Debug, Subcommand)]
pub enum Commands {
    #[command(about = "Manage authentication")]
    Auth(auth::AuthCommand),
    #[command(about = "Generate images through the SeaCloud proxy")]
    Images(images::ImagesCommand),
    #[command(about = "Browse available models")]
    Models(models::ModelsCommand),
    #[command(about = "Run a model and wait for the result")]
    Run(run::RunArgs),
    #[command(about = "Manage agent skills from SkillHub")]
    Skills(skills::SkillsCommand),
    #[command(about = "Manage generation tasks")]
    Task(task::TaskCommand),
    #[command(about = "Show CLI version")]
    Version(version::VersionArgs),
}

#[derive(Debug, Clone, Copy)]
pub struct Context {
    pub dry_run: bool,
}

pub fn execute() -> anyhow::Result<()> {
    let cli = Cli::parse();
    if cli.version {
        println!("seacloud version {}", buildinfo::VERSION);
        return Ok(());
    }

    let ctx = Context {
        dry_run: cli.dry_run,
    };
    match cli.command {
        Some(Commands::Auth(cmd)) => auth::handle(cmd, ctx),
        Some(Commands::Images(cmd)) => images::handle(cmd, ctx),
        Some(Commands::Models(cmd)) => models::handle(cmd, ctx),
        Some(Commands::Run(args)) => run::handle(args, ctx),
        Some(Commands::Skills(cmd)) => skills::handle(cmd, ctx),
        Some(Commands::Task(cmd)) => task::handle(cmd, ctx),
        Some(Commands::Version(args)) => version::handle(args, ctx),
        None => {
            Cli::command().print_help()?;
            println!();
            Ok(())
        }
    }
}
