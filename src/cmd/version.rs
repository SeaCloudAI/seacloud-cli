use super::root::Context;
use crate::internal::buildinfo;
use clap::Args;

#[derive(Debug, Args)]
pub struct VersionArgs {}

pub fn handle(_args: VersionArgs, _ctx: Context) -> anyhow::Result<()> {
    println!("{}", buildinfo::VERSION);
    Ok(())
}
