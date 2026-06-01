use super::agent_detector::detect_all_installed_agents;
use super::client::{load_config, save_config, Client};
use colored::Colorize;
use sha2::{Digest, Sha256};
use std::fs;
use std::io::{self, Write};
use std::path::{Path, PathBuf};

impl Client {
    pub fn find(
        &self,
        query: &str,
        category: &str,
        interactive: bool,
        cursor: &str,
    ) -> anyhow::Result<()> {
        if interactive {
            anyhow::bail!("interactive mode not implemented yet");
        }

        let display_query = if query.is_empty() { "all" } else { query };
        if !category.is_empty() {
            println!("🔍 Searching for \"{display_query}\" in category \"{category}\"\n");
        } else {
            println!("🔍 Searching for \"{display_query}\"\n");
        }

        let result = self.search_skills(query, category, cursor)?;
        if result.results.is_empty() {
            println!("No skills found.");
            return Ok(());
        }

        println!("Found {} result(s)\n", result.results.len());
        for skill in result.results {
            println!("{}", skill.display_name.cyan());
            println!("  {}", skill.description);
            println!("  {} • {}", "slug:".bright_black(), skill.slug.yellow());
            println!();
        }

        if !result.next_cursor.is_empty() {
            println!("To view more results, use: --cursor {}", result.next_cursor);
        }

        Ok(())
    }

    pub fn list(&self, category: &str, _sort: &str) -> anyhow::Result<()> {
        self.find("", category, false, "")
    }

    pub fn add(
        &self,
        slug: &str,
        version: &str,
        _global: bool,
        skip_confirm: bool,
    ) -> anyhow::Result<()> {
        println!("{} {}", "📦 Installing skill:".bold(), slug.cyan());

        let detail = self.get_skill_detail(slug)?;
        let install_version = if version.is_empty() {
            detail.latest_version.version.as_str()
        } else {
            version
        };

        println!(
            "{} {} v{}",
            "▸".bold(),
            detail.skill.display_name.bold(),
            install_version.green()
        );
        println!("  {}\n", detail.skill.description);

        println!("{}", "⬇ Downloading...".bold());
        let zip_data = self
            .download_skill(slug, install_version)
            .map_err(|err| anyhow::anyhow!("Download skill failed: {err}"))?;

        let hash = format!("{:x}", Sha256::digest(&zip_data));
        println!("  {}", format!("SHA256: {hash}").bright_black());

        let agents = detect_all_installed_agents();
        if !agents.is_empty() {
            let names: Vec<_> = agents
                .iter()
                .map(|agent| agent.display_name.as_str())
                .collect();
            println!(
                "  {} {}",
                "Detected agents:".bright_black(),
                names.join(", ").cyan()
            );
        }

        let home = home_dir();
        let global_repo = home.join(".agents").join("skills").join(slug);
        if global_repo.exists() {
            if !skip_confirm {
                print!("Skill {} already exists. Overwrite? [y/N] ", slug.cyan());
                io::stdout().flush()?;
                let mut answer = String::new();
                io::stdin().read_line(&mut answer)?;
                if !answer.trim().eq_ignore_ascii_case("y") {
                    println!("{}", "Installation cancelled.".yellow());
                    return Ok(());
                }
            }
            let _ = fs::remove_dir_all(&global_repo);
        }

        println!("{}", "📂 Extracting to global repository...".bold());
        extract_zip(&zip_data, &global_repo)
            .map_err(|err| anyhow::anyhow!("Extract failed: {err}"))?;

        if cfg!(windows) {
            println!("{}", "📋 Copying to agent directories...".bold());
        } else {
            println!("{}", "🔗 Creating symlinks...".bold());
        }

        let mut linked_count = 0usize;
        for agent in agents {
            let link_path = agent.global_skills_dir.join(slug);
            if link_path == global_repo {
                continue;
            }
            if let Err(err) = fs::create_dir_all(&agent.global_skills_dir) {
                println!(
                    "  {} Failed to create directory for {}: {}",
                    "⚠".yellow(),
                    agent.display_name,
                    err
                );
                continue;
            }
            let _ = remove_path(&link_path);
            if let Err(err) = link_or_copy_dir(&global_repo, &link_path) {
                println!(
                    "  {} Failed to link to {}: {}",
                    "⚠".yellow(),
                    agent.display_name,
                    err
                );
            } else {
                println!(
                    "  {} {} -> {}",
                    "✓".green(),
                    agent.display_name.cyan(),
                    link_path.display().to_string().bright_black()
                );
                linked_count += 1;
            }
        }

        println!();
        println!("{}", "✅ Installation complete!".bold().green());
        println!(
            "  {} {}",
            "Global repository:".bright_black(),
            global_repo.display().to_string().cyan()
        );
        if linked_count > 0 {
            let label = if cfg!(windows) {
                "Copies:"
            } else {
                "Symlinks:"
            };
            println!(
                "  {} Linked to {} agent(s)",
                label.bright_black(),
                linked_count
            );
        }
        println!();

        println!("{}", "Usage:".bold());
        println!("  The skill is now available in all your agent sessions.");
        println!("  Restart your agent if needed to load the new skill.");

        Ok(())
    }

    pub fn config(&self, set_url: &str, show: bool) -> anyhow::Result<()> {
        let mut config = load_config()?;
        if !set_url.is_empty() {
            config.api_base_url = set_url.to_string();
            save_config(&config)?;
            println!("{} API URL updated to: {}", "✓".green(), set_url.cyan());
            return Ok(());
        }

        if show {
            println!("{}", "Current Configuration:".bold());
            println!(
                "  {} {}",
                "API URL:".bright_black(),
                config.api_base_url.cyan()
            );
            println!(
                "  {} {}",
                "Config file:".bright_black(),
                super::client::config_file_path()
                    .display()
                    .to_string()
                    .cyan()
            );
            return Ok(());
        }

        anyhow::bail!("use --set-url or --show")
    }
}

fn home_dir() -> PathBuf {
    std::env::var_os("HOME")
        .map(PathBuf::from)
        .or_else(dirs::home_dir)
        .unwrap_or_else(|| PathBuf::from("."))
}

fn remove_path(path: &Path) -> anyhow::Result<()> {
    if !path.exists() {
        return Ok(());
    }
    let metadata = fs::symlink_metadata(path)?;
    if metadata.is_dir() && !metadata.file_type().is_symlink() {
        fs::remove_dir_all(path)?;
    } else {
        fs::remove_file(path)?;
    }
    Ok(())
}

fn link_or_copy_dir(src: &Path, dst: &Path) -> anyhow::Result<()> {
    if cfg!(windows) {
        copy_dir(src, dst)
    } else {
        #[cfg(unix)]
        {
            std::os::unix::fs::symlink(src, dst)?;
            Ok(())
        }
        #[cfg(not(unix))]
        {
            copy_dir(src, dst)
        }
    }
}

fn copy_dir(src: &Path, dst: &Path) -> anyhow::Result<()> {
    let metadata = fs::metadata(src)?;
    fs::create_dir_all(dst)?;
    fs::set_permissions(dst, metadata.permissions())?;
    for entry in fs::read_dir(src)? {
        let entry = entry?;
        let src_path = entry.path();
        let dst_path = dst.join(entry.file_name());
        if entry.file_type()?.is_dir() {
            copy_dir(&src_path, &dst_path)?;
        } else {
            copy_file(&src_path, &dst_path)?;
        }
    }
    Ok(())
}

fn copy_file(src: &Path, dst: &Path) -> anyhow::Result<()> {
    if let Some(parent) = dst.parent() {
        fs::create_dir_all(parent)?;
    }
    fs::copy(src, dst)?;
    fs::set_permissions(dst, fs::metadata(src)?.permissions())?;
    Ok(())
}

fn extract_zip(zip_data: &[u8], target_dir: &Path) -> anyhow::Result<()> {
    let reader = std::io::Cursor::new(zip_data);
    let mut archive = zip::ZipArchive::new(reader)?;

    let mut top_level_dirs = std::collections::BTreeSet::new();
    for i in 0..archive.len() {
        let file = archive.by_index(i)?;
        if let Some(first) = file.name().split('/').find(|part| !part.is_empty()) {
            top_level_dirs.insert(first.to_string());
        }
    }
    let strip_prefix = if top_level_dirs.len() == 1 {
        top_level_dirs.iter().next().map(|dir| format!("{dir}/"))
    } else {
        None
    };

    for i in 0..archive.len() {
        let mut file = archive.by_index(i)?;
        let mut file_name = file.name().to_string();
        if let Some(prefix) = &strip_prefix {
            file_name = file_name
                .strip_prefix(prefix)
                .unwrap_or(&file_name)
                .to_string();
        }
        if file_name.is_empty() {
            continue;
        }
        let target_path = target_dir.join(&file_name);
        if !target_path.starts_with(target_dir) {
            anyhow::bail!("zip entry escapes target directory: {file_name}");
        }

        if file.is_dir() {
            fs::create_dir_all(&target_path)?;
            continue;
        }

        if let Some(parent) = target_path.parent() {
            fs::create_dir_all(parent)?;
        }
        let mut out = fs::File::create(&target_path)?;
        io::copy(&mut file, &mut out)?;
    }

    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::internal::test_support::{TestResponse, TestServer};
    use serial_test::serial;
    use std::env;
    use tempfile::TempDir;
    use zip::write::FileOptions;

    fn zip_with_top_level() -> Vec<u8> {
        let cursor = std::io::Cursor::new(Vec::new());
        let mut zip = zip::ZipWriter::new(cursor);
        zip.add_directory("skill/", FileOptions::default()).unwrap();
        zip.start_file("skill/SKILL.md", FileOptions::default())
            .unwrap();
        zip.write_all(b"# Skill").unwrap();
        zip.finish().unwrap().into_inner()
    }

    #[test]
    fn extract_zip_strips_single_top_level_directory_and_copy_file() {
        let target = TempDir::new().unwrap();
        extract_zip(&zip_with_top_level(), target.path()).unwrap();
        assert_eq!(
            fs::read_to_string(target.path().join("SKILL.md")).unwrap(),
            "# Skill"
        );

        let copied = target.path().join("copy.md");
        copy_file(&target.path().join("SKILL.md"), &copied).unwrap();
        assert_eq!(fs::read_to_string(copied).unwrap(), "# Skill");
    }

    #[test]
    fn copy_dir_and_link_or_copy_dir_work_for_directories() {
        let root = TempDir::new().unwrap();
        let src = root.path().join("src");
        let dst = root.path().join("dst");
        fs::create_dir_all(&src).unwrap();
        fs::write(src.join("file.txt"), "ok").unwrap();
        copy_dir(&src, &dst).unwrap();
        assert_eq!(fs::read_to_string(dst.join("file.txt")).unwrap(), "ok");

        let linked = root.path().join("linked");
        link_or_copy_dir(&src, &linked).unwrap();
        assert!(linked.exists());
        remove_path(&linked).unwrap();
        assert!(!linked.exists());
    }

    #[test]
    #[serial]
    fn find_list_config_and_add_happy_path() {
        let home = TempDir::new().unwrap();
        env::set_var("HOME", home.path());
        fs::create_dir_all(home.path().join(".codex")).unwrap();
        let zip = zip_with_top_level();
        let server = TestServer::new(move |req| match req.path.as_str() {
            "/search?q=&limit=20" => TestResponse::json(
                200,
                r#"{"results":[{"slug":"cat","displayName":"Cat","description":"desc"}]}"#,
            ),
            "/skills/cat" => TestResponse::json(
                200,
                r#"{"skill":{"slug":"cat","displayName":"Cat","description":"desc"},"latestVersion":{"version":"1.0.0"}}"#,
            ),
            "/skills/cat/download?version=1.0.0" => {
                TestResponse::bytes(200, zip.clone(), "application/zip")
            }
            other => panic!("unexpected path {other}"),
        });
        env::set_var("SEACLOUD_SKILLHUB_URL", server.url());
        let client = Client::new();
        client.find("", "", false, "").unwrap();
        client.list("", "").unwrap();
        client
            .config("https://skillhub.example.com/api", false)
            .unwrap();
        client.config("", true).unwrap();
        client.add("cat", "", true, true).unwrap();
        assert!(home.path().join(".agents/skills/cat/SKILL.md").exists());
    }

    #[test]
    fn interactive_and_empty_config_errors_are_reported() {
        let client = Client::new();
        assert!(client
            .find("", "", true, "")
            .unwrap_err()
            .to_string()
            .contains("interactive"));
        assert!(client
            .config("", false)
            .unwrap_err()
            .to_string()
            .contains("use --set-url"));
    }
}
