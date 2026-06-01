use super::images::execute_sync_image_request;
use super::root::Context;
use crate::internal::{clierrors, config, generation, images as imageapi, models};
use clap::{ArgAction, Args};
use std::time::Duration;

#[derive(Debug, Args)]
pub struct RunArgs {
    model_id: String,
    #[arg(long = "param", action = ArgAction::Append, help = "Parameter as key=value (repeatable)")]
    params: Vec<String>,
    #[arg(
        long,
        default_value = "",
        help = "Output format: url (URLs only), json (full response)"
    )]
    output: String,
    #[arg(
        long,
        default_value_t = 600,
        help = "Maximum seconds to wait for result (default 10 minutes)"
    )]
    timeout: u64,
}

pub fn handle(args: RunArgs, ctx: Context) -> anyhow::Result<()> {
    let resolved_model_id = models::resolve_model_id(&args.model_id);

    if imageapi::client::supports_sync_model(&resolved_model_id) {
        let raw = generation::parse_params(&args.params)?;
        let req = imageapi::client::request_from_params(&args.model_id, &raw)?;
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
        return execute_sync_image_request(
            &cfg.api_key,
            req,
            &args.output,
            Duration::from_secs(args.timeout),
        );
    }

    if ctx.dry_run {
        eprintln!("[dry-run] Would execute: POST <spec.api.endpoint>");
        eprintln!("[dry-run] model={} params={:?}", args.model_id, args.params);
        eprintln!(
            "[dry-run] Fetch spec first with: seacloud models spec {}",
            args.model_id
        );
        return Ok(());
    }

    let cfg = config::load()?;
    if cfg.api_key.is_empty() {
        return Err(clierrors::err_no_api_key().into());
    }

    let spec = models::get_spec(&args.model_id)?;
    let raw = generation::parse_params(&args.params)?;
    let params = generation::validate_and_coerce(&args.model_id, &raw, &spec.parameters)?;
    let resp = generation::submit(&cfg.api_key, &spec.api.endpoint, &resolved_model_id, params)
        .map_err(clierrors::err_submit_failed)?;

    eprintln!("Task submitted: {}\nWaiting for result...", resp.id);

    let timeout = Duration::from_secs(args.timeout);
    let mut last_progress = -1.0f64;
    let task_result = generation::poll_task(
        &cfg.api_key,
        &spec.api.endpoint,
        &resp.id,
        generation::DEFAULT_POLL_INTERVAL,
        timeout,
        |progress| {
            let pct = (progress * 100.0) as i64;
            if (pct as f64) - last_progress >= 5.0 || (last_progress < 0.0 && pct == 0) {
                eprintln!("Progress: {pct}%");
                last_progress = pct as f64;
            }
        },
    );

    let mut task = match task_result {
        Ok(task) => task,
        Err(_) => return Err(clierrors::err_task_timeout(&resp.id).into()),
    };

    if task.status == "failed" {
        let reason = task
            .error
            .as_ref()
            .map(|err| err.message.as_str())
            .filter(|value| !value.is_empty())
            .unwrap_or("unknown error");
        return Err(clierrors::err_task_failed(&resp.id, reason).into());
    }

    if task.model == resolved_model_id {
        task.model = args.model_id.clone();
    }

    match args.output.as_str() {
        "url" => {
            for url in task.urls() {
                println!("{url}");
            }
        }
        "json" => {
            println!("{}", serde_json::to_string_pretty(&task)?);
        }
        "" => {
            println!("Status: {}", task.status);
            for group in task.output {
                for content in group.content {
                    if !content.url.is_empty() {
                        println!("URL: {}", content.url);
                    }
                    if content.img_id != 0 {
                        println!("ImgID: {}", content.img_id);
                    }
                }
            }
        }
        _ => {}
    }
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::internal::test_support::{TestResponse, TestServer};
    use serial_test::serial;
    use std::env;

    #[test]
    fn dry_run_covers_sync_and_async_paths() {
        handle(
            RunArgs {
                model_id: "seedance_2_0".to_string(),
                params: vec!["prompt=test".to_string()],
                output: String::new(),
                timeout: 1,
            },
            Context { dry_run: true },
        )
        .unwrap();
        handle(
            RunArgs {
                model_id: "gpt-image-2".to_string(),
                params: vec!["prompt=cat".to_string()],
                output: "url".to_string(),
                timeout: 1,
            },
            Context { dry_run: true },
        )
        .unwrap();
    }

    #[test]
    #[serial]
    fn async_run_submits_polls_and_outputs_modes() {
        let server = TestServer::new(|req| {
            if req.path == "/api/v1/skill/models/kirin_v2_6_i2v/spec" {
                return TestResponse::json(
                    200,
                    r#"{"status":{"code":200,"message":"ok"},"data":{"model_id":"kirin_v2_6_i2v","name":"Kling","vendor":"kling","type":"video","api":{"endpoint":"http://127.0.0.1:9/model/v1/generation","method":"POST","headers":{}},"parameters":[{"name":"prompt","type":"string","required":true}],"agent_prompt":"prompt"}}"#,
                );
            }
            TestResponse::json(500, "{}")
        });
        let generation = TestServer::new(|req| {
            if req.path == "/model/v1/generation" {
                return TestResponse::json(
                    200,
                    r#"{"id":"task-1","status":"in_progress","model":"kirin_v2_6_i2v"}"#,
                );
            }
            assert_eq!(req.path, "/model/v1/generation/task/task-1");
            TestResponse::json(
                200,
                r#"{"id":"task-1","status":"completed","model":"kirin_v2_6_i2v","progress":1,"output":[{"content":[{"url":"https://cdn.example.com/out.mp4","img_id":12}]}]}"#,
            )
        });
        env::set_var("SEACLOUD_MODELS_URL", server.url());
        env::set_var(config::ENV_FOLKOS_EXEC_TOKEN, "managed-token");

        let endpoint = generation.url() + "/model/v1/generation";
        let spec_server = TestServer::new(move |req| {
            assert_eq!(req.path, "/api/v1/skill/models/kirin_v2_6_i2v/spec");
            TestResponse::json(
                200,
                format!(
                    r#"{{"status":{{"code":200,"message":"ok"}},"data":{{"model_id":"kirin_v2_6_i2v","name":"Kling","vendor":"kling","type":"video","api":{{"endpoint":"{}","method":"POST","headers":{{}}}},"parameters":[{{"name":"prompt","type":"string","required":true}}],"agent_prompt":"prompt"}}}}"#,
                    endpoint
                ),
            )
        });
        env::set_var("SEACLOUD_MODELS_URL", spec_server.url());

        handle(
            RunArgs {
                model_id: "kling_v2_6_i2v".to_string(),
                params: vec!["prompt=test".to_string()],
                output: String::new(),
                timeout: 1,
            },
            Context { dry_run: false },
        )
        .unwrap();
    }
}
