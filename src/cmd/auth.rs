use super::root::Context;
use crate::internal::{auth, clierrors, config};
use clap::{Args, Subcommand};
use std::process::Command;

#[derive(Debug, Args)]
pub struct AuthCommand {
    #[command(subcommand)]
    command: AuthSubcommand,
}

#[derive(Debug, Subcommand)]
enum AuthSubcommand {
    #[command(about = "Log in to your SeaCloud account")]
    Login,
    #[command(about = "Show current authentication status")]
    Status,
    #[command(about = "Log out of your SeaCloud account")]
    Logout,
    #[command(name = "set-key", about = "Set or replace the SeaCloud API key")]
    SetKey { api_key: String },
}

pub fn handle(cmd: AuthCommand, _ctx: Context) -> anyhow::Result<()> {
    match cmd.command {
        AuthSubcommand::Login => login(),
        AuthSubcommand::Status => status(),
        AuthSubcommand::Logout => logout(),
        AuthSubcommand::SetKey { api_key } => set_key(&api_key),
    }
}

fn login() -> anyhow::Result<()> {
    let cfg = config::load()?;
    if cfg.managed {
        println!(
            "Authentication is managed by the {} runtime. Interactive login skipped.",
            managed_runtime_name(&cfg)
        );
        return Ok(());
    }
    if !cfg.auth_token.is_empty() {
        if let Ok(me) = auth::verify_token(&cfg.auth_token) {
            println!(
                "Already logged in as {}. Run seacloud auth logout to switch accounts.",
                me.email
            );
            return Ok(());
        }
    }

    let (token, refresh_token, api_key) = auth::login(open_browser)?;
    let me = auth::verify_token(&token).map_err(clierrors::err_token_verification)?;
    config::save(&config::Config {
        auth_token: token,
        refresh_token,
        api_key,
        ..config::Config::default()
    })
    .map_err(clierrors::err_save_config)?;

    let email = if me.email.is_empty() {
        me.account
    } else {
        me.email
    };
    println!("\nLogged in as {email}");
    Ok(())
}

fn status() -> anyhow::Result<()> {
    let cfg = config::load().map_err(|err| anyhow::anyhow!("failed to read config: {err}"))?;
    if cfg.managed {
        println!(
            "Authenticated via managed runtime: {}",
            managed_runtime_name(&cfg)
        );
        if !cfg.credential_source.is_empty() {
            println!("Credential source: {}", cfg.credential_source);
        }
        println!("Interactive login is disabled in this environment.");
        return Ok(());
    }

    if cfg.auth_token.is_empty() {
        println!("Not logged in.");
        println!("  Hint: Run: seacloud auth login");
        return Ok(());
    }

    let me = match auth::verify_token(&cfg.auth_token) {
        Ok(me) => me,
        Err(_) => {
            println!("Token expired or invalid.");
            println!("  Hint: Run: seacloud auth login");
            return Ok(());
        }
    };

    let email = if me.email.is_empty() {
        me.account
    } else {
        me.email
    };
    println!("Logged in as {email}");
    if !me.name.is_empty() {
        println!("Name: {}", me.name);
    }
    Ok(())
}

fn logout() -> anyhow::Result<()> {
    let cfg = config::load()?;
    if cfg.managed {
        println!(
            "Authentication is managed by the {} runtime. No local credentials were removed.",
            managed_runtime_name(&cfg)
        );
        return Ok(());
    }
    config::clear().map_err(clierrors::err_logout)?;
    println!("Logged out.");
    Ok(())
}

fn set_key(new_key: &str) -> anyhow::Result<()> {
    if new_key.is_empty() {
        anyhow::bail!("API key cannot be empty");
    }

    let mut cfg = match config::load() {
        Ok(cfg) => cfg,
        Err(_) => config::load_stored().unwrap_or_default(),
    };
    if cfg.managed {
        return Err(clierrors::err_managed_credentials_override().into());
    }

    cfg.api_key = new_key.to_string();
    config::save(&cfg).map_err(clierrors::err_save_config)?;
    println!("API key updated.");
    Ok(())
}

fn open_browser(url: &str) -> anyhow::Result<()> {
    let (program, args): (&str, Vec<&str>) = if cfg!(target_os = "macos") {
        ("open", vec![url])
    } else if cfg!(target_os = "windows") {
        ("rundll32", vec!["url.dll,FileProtocolHandler", url])
    } else if cfg!(target_os = "linux") {
        ("xdg-open", vec![url])
    } else {
        anyhow::bail!("unsupported operating system");
    };

    Command::new(program).args(args).spawn()?;
    Ok(())
}

fn managed_runtime_name(cfg: &config::Config) -> &str {
    if cfg.runtime.is_empty() {
        "managed"
    } else {
        &cfg.runtime
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use serial_test::serial;
    use tempfile::TempDir;

    #[test]
    #[serial]
    fn managed_auth_commands_do_not_touch_local_state() {
        let home = TempDir::new().unwrap();
        std::env::set_var("HOME", home.path());
        std::env::set_var("SEACLOUD_NO_KEYCHAIN", "1");
        std::env::set_var(config::ENV_FOLKOS_EXEC_TOKEN, "managed-token");
        std::env::set_var(config::ENV_SEACLOUD_RUNTIME, config::RUNTIME_FOLKOS);

        login().unwrap();
        status().unwrap();
        logout().unwrap();
        assert!(set_key("new-key").is_err());
    }

    #[test]
    #[serial]
    fn local_set_key_status_and_logout_round_trip() {
        let home = TempDir::new().unwrap();
        std::env::set_var("HOME", home.path());
        std::env::set_var("SEACLOUD_NO_KEYCHAIN", "1");
        std::env::remove_var(config::ENV_FOLKOS_EXEC_TOKEN);
        std::env::remove_var(config::ENV_FOLKOS_TOKEN);
        std::env::remove_var(config::ENV_SEACLOUD_RUNTIME);

        assert!(set_key("").is_err());
        set_key("local-key").unwrap();
        let cfg = config::load_stored().unwrap();
        assert_eq!(cfg.api_key, "local-key");
        status().unwrap();
        logout().unwrap();
    }

    #[test]
    fn managed_runtime_name_defaults_when_runtime_empty() {
        assert_eq!(managed_runtime_name(&config::Config::default()), "managed");
    }
}
