use super::root::Context;
use crate::internal::{clierrors, config, images as imageapi};
use clap::{Args, Subcommand};
use std::time::Duration;

#[derive(Debug, Args)]
pub struct ImagesCommand {
    #[command(subcommand)]
    command: ImagesSubcommand,
}

#[derive(Debug, Subcommand)]
enum ImagesSubcommand {
    #[command(about = "Generate an image through the proxy-backed image API")]
    Generate(ImagesGenerateArgs),
}

#[derive(Debug, Args)]
struct ImagesGenerateArgs {
    #[arg(long, default_value = imageapi::DEFAULT_MODEL, help = "Image model ID")]
    model: String,
    #[arg(long, default_value = "", help = "Prompt to generate")]
    prompt: String,
    #[arg(long, default_value = imageapi::DEFAULT_SIZE, help = "Image size")]
    size: String,
    #[arg(long = "response-format", default_value = imageapi::DEFAULT_RESPONSE_FORMAT, help = "Response format")]
    response_format: String,
    #[arg(
        long,
        default_value = "",
        help = "Output format: url (assets CDN URLs), json (full response)"
    )]
    output: String,
    #[arg(long, default_value_t = imageapi::DEFAULT_TIMEOUT.as_secs(), help = "Maximum seconds to wait for image generation")]
    timeout: u64,
}

pub fn handle(cmd: ImagesCommand, ctx: Context) -> anyhow::Result<()> {
    match cmd.command {
        ImagesSubcommand::Generate(args) => generate(args, ctx),
    }
}

fn generate(args: ImagesGenerateArgs, ctx: Context) -> anyhow::Result<()> {
    let req = imageapi::client::request_from_values(
        &args.model,
        &args.prompt,
        &args.size,
        &args.response_format,
    )?;

    if ctx.dry_run {
        eprintln!(
            "[dry-run] Would execute: POST <proxy>{}",
            imageapi::ROUTE_GENERATE
        );
        eprintln!("[dry-run] request={req:?}");
        return Ok(());
    }

    let cfg = config::load()?;
    if cfg.api_key.is_empty() {
        return Err(clierrors::err_no_api_key().into());
    }
    execute_sync_image_request(
        &cfg.api_key,
        req,
        &args.output,
        Duration::from_secs(args.timeout),
    )
}

pub fn execute_sync_image_request(
    api_key: &str,
    req: imageapi::GenerateRequest,
    output: &str,
    timeout: Duration,
) -> anyhow::Result<()> {
    let client = imageapi::Client::new_with_timeout(api_key, timeout);
    let resp = client.generate(req).map_err(clierrors::err_submit_failed)?;

    match output {
        "json" => {
            println!("{}", serde_json::to_string_pretty(&resp)?);
            Ok(())
        }
        "url" => {
            let urls = client
                .upload_response_images(&resp)
                .map_err(clierrors::err_submit_failed)?;
            for url in urls {
                println!("{url}");
            }
            Ok(())
        }
        "" => {
            println!("{}", imageapi::summary(&resp));
            Ok(())
        }
        other => anyhow::bail!("unsupported output format {other:?}"),
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::internal::test_support::{TestResponse, TestServer};
    use serial_test::serial;
    use std::env;

    #[test]
    fn generate_dry_run_and_validation_paths() {
        generate(
            ImagesGenerateArgs {
                model: "gpt-image-2".to_string(),
                prompt: "cat".to_string(),
                size: "1024x1024".to_string(),
                response_format: "b64_json".to_string(),
                output: String::new(),
                timeout: 1,
            },
            Context { dry_run: true },
        )
        .unwrap();

        assert!(generate(
            ImagesGenerateArgs {
                model: "gpt-image-2".to_string(),
                prompt: String::new(),
                size: String::new(),
                response_format: String::new(),
                output: String::new(),
                timeout: 1,
            },
            Context { dry_run: true },
        )
        .is_err());
    }

    #[test]
    #[serial]
    fn execute_sync_image_request_covers_output_modes() {
        env::remove_var(imageapi::client::ENV_PROXY_URL);
        let server = TestServer::new(|req| {
            if req.path == imageapi::client::ROUTE_GENERATE {
                return TestResponse::json(
                    200,
                    r#"{"data":[{"url":"https://cdn.example.com/cat.png"}],"output_format":"png","size":"1024x1024"}"#,
                );
            }
            TestResponse::json(500, "{}")
        });
        env::set_var(imageapi::client::ENV_PROXY_URL, server.url());
        let req = imageapi::client::request_from_values("gpt-image-2", "cat", "", "").unwrap();
        execute_sync_image_request("key", req.clone(), "", Duration::from_secs(1)).unwrap();
        execute_sync_image_request("key", req.clone(), "json", Duration::from_secs(1)).unwrap();
        execute_sync_image_request("key", req, "url", Duration::from_secs(1)).unwrap();

        let req = imageapi::client::request_from_values("gpt-image-2", "cat", "", "").unwrap();
        assert!(execute_sync_image_request("key", req, "bad", Duration::from_secs(1)).is_err());
    }
}
