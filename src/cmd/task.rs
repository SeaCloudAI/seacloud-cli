use super::root::Context;
use crate::internal::{clierrors, config, generation};
use clap::{Args, Subcommand};

#[derive(Debug, Args)]
pub struct TaskCommand {
    #[command(subcommand)]
    command: TaskSubcommand,
}

#[derive(Debug, Subcommand)]
enum TaskSubcommand {
    #[command(about = "Get the current status of a generation task")]
    Status(TaskStatusArgs),
}

#[derive(Debug, Args)]
struct TaskStatusArgs {
    task_id: String,
    #[arg(
        long,
        default_value = "",
        help = "Output format: url (URLs only), json (full response)"
    )]
    output: String,
}

pub fn handle(cmd: TaskCommand, _ctx: Context) -> anyhow::Result<()> {
    match cmd.command {
        TaskSubcommand::Status(args) => status(args),
    }
}

fn status(args: TaskStatusArgs) -> anyhow::Result<()> {
    let cfg = config::load()?;
    if cfg.api_key.is_empty() {
        return Err(clierrors::err_no_api_key().into());
    }

    let task = generation::get_task(&cfg.api_key, &args.task_id)
        .map_err(|err| anyhow::anyhow!("failed to fetch task {}: {err}", args.task_id))?;

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
            println!("Task:   {}", task.id);
            println!("Status: {}", task.status);
            if task.status == "failed" {
                if let Some(error) = &task.error {
                    println!("Error:  {}", error.message);
                }
            }
            for url in task.urls() {
                println!("URL:    {url}");
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
    #[serial]
    fn status_command_covers_human_json_url_and_failed() {
        let server = TestServer::new(|req| {
            assert!(req.path.starts_with("/model/v1/generation/task/"));
            if req.path.ends_with("failed") {
                return TestResponse::json(
                    200,
                    r#"{"id":"failed","status":"failed","error":"bad"}"#,
                );
            }
            TestResponse::json(
                200,
                r#"{"id":"task-1","status":"completed","output":[{"content":[{"url":"https://cdn.example.com/out.png"}]}]}"#,
            )
        });
        env::set_var("SEACLOUD_GENERATION_URL", server.url());
        env::set_var(config::ENV_FOLKOS_EXEC_TOKEN, "managed-token");

        status(TaskStatusArgs {
            task_id: "task-1".to_string(),
            output: String::new(),
        })
        .unwrap();
        status(TaskStatusArgs {
            task_id: "task-1".to_string(),
            output: "json".to_string(),
        })
        .unwrap();
        status(TaskStatusArgs {
            task_id: "task-1".to_string(),
            output: "url".to_string(),
        })
        .unwrap();
        status(TaskStatusArgs {
            task_id: "failed".to_string(),
            output: String::new(),
        })
        .unwrap();
    }
}
