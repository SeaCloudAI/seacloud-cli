use super::root::Context;
use crate::internal::models;
use clap::{Args, Subcommand};

#[derive(Debug, Args)]
pub struct ModelsCommand {
    #[command(subcommand)]
    command: ModelsSubcommand,
}

#[derive(Debug, Subcommand)]
enum ModelsSubcommand {
    #[command(about = "List available models")]
    List(ModelsListArgs),
    #[command(about = "Get full parameter spec for a model")]
    Spec(ModelsSpecArgs),
}

#[derive(Debug, Args)]
struct ModelsListArgs {
    #[arg(long = "type", help = "Filter by type (video, image, audio)")]
    model_type: Option<String>,
    #[arg(long, help = "Search by keyword")]
    keywords: Option<String>,
    #[arg(long, default_value_t = 1, help = "Page number")]
    page: usize,
    #[arg(long = "page-size", default_value_t = 20, help = "Results per page")]
    page_size: usize,
    #[arg(
        long,
        default_value = "",
        help = "Output format: id (IDs only), json (full response)"
    )]
    output: String,
}

#[derive(Debug, Args)]
struct ModelsSpecArgs {
    model_id: String,
    #[arg(long, default_value = "", help = "Output format (json)")]
    output: String,
}

pub fn handle(cmd: ModelsCommand, _ctx: Context) -> anyhow::Result<()> {
    match cmd.command {
        ModelsSubcommand::List(args) => list(args),
        ModelsSubcommand::Spec(args) => spec(args),
    }
}

fn list(args: ModelsListArgs) -> anyhow::Result<()> {
    let result = models::list(models::ListParams {
        page: args.page,
        page_size: args.page_size,
        model_type: args.model_type.unwrap_or_default(),
        keywords: args.keywords.unwrap_or_default(),
    })?;

    if args.output == "json" {
        println!("{}", serde_json::to_string_pretty(&result)?);
        return Ok(());
    }
    if args.output == "id" {
        for model in result.models {
            println!("{}", model.id);
        }
        return Ok(());
    }
    if result.models.is_empty() {
        println!("No models found.");
        return Ok(());
    }

    println!(
        "Showing {} of {} models (page {}/{})\n",
        result.models.len(),
        result.total,
        result.page,
        result.total_pages
    );
    for model in result.models {
        println!("{:<30}  {:<8}  {}", model.id, model.model_type, model.name);
        if !model.description.is_empty() {
            println!("  {}", truncate(&model.description, 80));
        }
        println!(
            "  Input: {}  →  Output: {}\n",
            model.input_modalities.join(", "),
            model.output_modalities.join(", ")
        );
    }
    Ok(())
}

fn spec(args: ModelsSpecArgs) -> anyhow::Result<()> {
    let spec = models::get_spec(&args.model_id)?;
    if args.output == "json" {
        println!("{}", serde_json::to_string_pretty(&spec)?);
        return Ok(());
    }
    println!("{}", spec.agent_prompt);
    Ok(())
}

fn truncate(s: &str, max: usize) -> String {
    if s.len() <= max {
        return s.to_string();
    }
    let take = max.saturating_sub(3);
    format!("{}...", s.chars().take(take).collect::<String>())
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::internal::test_support::{TestResponse, TestServer};
    use serial_test::serial;
    use std::env;

    #[test]
    fn truncates_long_descriptions() {
        assert_eq!(truncate("short", 10), "short");
        assert_eq!(truncate("abcdef", 5), "ab...");
    }

    #[test]
    #[serial]
    fn list_and_spec_commands_cover_output_modes() {
        let server = TestServer::new(|req| {
            if req.path.starts_with("/api/v1/skill/models?") || req.path == "/api/v1/skill/models" {
                return TestResponse::json(
                    200,
                    r#"{"status":{"code":200,"message":"ok"},"data":{"models":[{"id":"kirin_v2_6_i2v","name":"Kling","type":"video","description":"a long model description for truncation","input_modalities":["image"],"output_modalities":["video"]}],"total":1,"page":1,"page_size":20,"total_pages":1}}"#,
                );
            }
            assert_eq!(req.path, "/api/v1/skill/models/kirin_v2_6_i2v/spec");
            TestResponse::json(
                200,
                r#"{"status":{"code":200,"message":"ok"},"data":{"model_id":"kirin_v2_6_i2v","name":"Kling","vendor":"kling","type":"video","api":{"endpoint":"https://example.com/model","method":"POST","headers":{}},"parameters":[],"agent_prompt":"prompt kirin_v2_6_i2v"}}"#,
            )
        });
        env::set_var("SEACLOUD_MODELS_URL", server.url());

        list(ModelsListArgs {
            model_type: Some("video".to_string()),
            keywords: Some("kling".to_string()),
            page: 1,
            page_size: 20,
            output: String::new(),
        })
        .unwrap();
        list(ModelsListArgs {
            model_type: None,
            keywords: None,
            page: 1,
            page_size: 20,
            output: "id".to_string(),
        })
        .unwrap();
        list(ModelsListArgs {
            model_type: None,
            keywords: None,
            page: 1,
            page_size: 20,
            output: "json".to_string(),
        })
        .unwrap();
        spec(ModelsSpecArgs {
            model_id: "kling_v2_6_i2v".to_string(),
            output: String::new(),
        })
        .unwrap();
        spec(ModelsSpecArgs {
            model_id: "kling_v2_6_i2v".to_string(),
            output: "json".to_string(),
        })
        .unwrap();
    }
}
