use std::env;
use std::path::PathBuf;

#[derive(Debug, Clone)]
#[allow(dead_code)]
pub struct AgentConfig {
    pub name: String,
    pub display_name: String,
    pub local_skills_dir: String,
    pub global_skills_dir: PathBuf,
}

fn home_dir() -> PathBuf {
    env::var_os("HOME")
        .map(PathBuf::from)
        .or_else(dirs::home_dir)
        .unwrap_or_else(|| PathBuf::from("."))
}

fn agent_configs() -> Vec<AgentConfig> {
    let home = home_dir();
    vec![
        AgentConfig {
            name: "cursor".to_string(),
            display_name: "Cursor".to_string(),
            local_skills_dir: ".agents/skills".to_string(),
            global_skills_dir: home.join(".cursor").join("skills"),
        },
        AgentConfig {
            name: "claude-code".to_string(),
            display_name: "Claude Code".to_string(),
            local_skills_dir: ".claude/skills".to_string(),
            global_skills_dir: home.join(".claude").join("skills"),
        },
        AgentConfig {
            name: "codex".to_string(),
            display_name: "Codex".to_string(),
            local_skills_dir: ".agents/skills".to_string(),
            global_skills_dir: home.join(".codex").join("skills"),
        },
        AgentConfig {
            name: "cline".to_string(),
            display_name: "Cline".to_string(),
            local_skills_dir: ".agents/skills".to_string(),
            global_skills_dir: home.join(".agents").join("skills"),
        },
        AgentConfig {
            name: "continue".to_string(),
            display_name: "Continue".to_string(),
            local_skills_dir: ".continue/skills".to_string(),
            global_skills_dir: home.join(".continue").join("skills"),
        },
        AgentConfig {
            name: "openclaw".to_string(),
            display_name: "OpenClaw".to_string(),
            local_skills_dir: "skills".to_string(),
            global_skills_dir: home.join(".openclaw").join("skills"),
        },
    ]
}

#[allow(dead_code)]
pub fn detect_current_agent() -> Option<AgentConfig> {
    let configs = agent_configs();

    if env::var("CURSOR_AGENT").unwrap_or_default() != "" {
        return configs.into_iter().find(|cfg| cfg.name == "cursor");
    }

    if env::var("CODEX_HOME").unwrap_or_default() != "" {
        return configs.into_iter().find(|cfg| cfg.name == "codex");
    }

    if env::var("PATH").unwrap_or_default().contains("claude-code")
        && env::var("CURSOR_AGENT").unwrap_or_default().is_empty()
    {
        return configs.into_iter().find(|cfg| cfg.name == "claude-code");
    }

    let home = home_dir();
    let agents = [
        ("cursor", home.join(".cursor")),
        ("claude-code", home.join(".claude")),
        ("codex", home.join(".codex")),
        ("cline", home.join(".cline")),
        ("continue", home.join(".continue")),
        ("openclaw", home.join(".openclaw")),
    ];

    for (name, path) in agents {
        if path.exists() {
            return agent_configs().into_iter().find(|cfg| cfg.name == name);
        }
    }

    None
}

#[allow(dead_code)]
pub fn get_install_dir(global: bool, agent: Option<&AgentConfig>) -> PathBuf {
    if global {
        if let Some(agent) = agent {
            return agent.global_skills_dir.clone();
        }
        return home_dir().join(".agents").join("skills");
    }

    let local_dir = agent
        .map(|agent| agent.local_skills_dir.as_str())
        .unwrap_or(".agents/skills");
    env::current_dir()
        .unwrap_or_else(|_| PathBuf::from("."))
        .join(local_dir)
}

pub fn detect_all_installed_agents() -> Vec<AgentConfig> {
    agent_configs()
        .into_iter()
        .filter(|cfg| {
            cfg.global_skills_dir
                .parent()
                .map(|path| path.exists())
                .unwrap_or(false)
        })
        .collect()
}

#[cfg(test)]
mod tests {
    use super::*;
    use serial_test::serial;
    use std::fs;
    use tempfile::TempDir;

    #[test]
    #[serial]
    fn detects_current_and_all_installed_agents() {
        let home = TempDir::new().unwrap();
        std::env::set_var("HOME", home.path());
        std::env::remove_var("CURSOR_AGENT");
        std::env::set_var("CODEX_HOME", home.path());
        assert_eq!(detect_current_agent().unwrap().name, "codex");

        std::env::remove_var("CODEX_HOME");
        fs::create_dir_all(home.path().join(".claude")).unwrap();
        assert_eq!(detect_current_agent().unwrap().name, "claude-code");
        let installed = detect_all_installed_agents();
        assert!(installed.iter().any(|agent| agent.name == "claude-code"));
    }

    #[test]
    #[serial]
    fn install_dir_uses_agent_or_defaults() {
        let home = TempDir::new().unwrap();
        std::env::set_var("HOME", home.path());
        let agent = AgentConfig {
            name: "test".to_string(),
            display_name: "Test".to_string(),
            local_skills_dir: "local/skills".to_string(),
            global_skills_dir: home.path().join("global/skills"),
        };
        assert_eq!(get_install_dir(true, Some(&agent)), agent.global_skills_dir);
        assert!(get_install_dir(false, Some(&agent)).ends_with("local/skills"));
        assert!(get_install_dir(true, None).ends_with(".agents/skills"));
    }
}
