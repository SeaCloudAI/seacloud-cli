use assert_cmd::Command;
use tempfile::TempDir;

fn seacloud() -> Command {
    Command::cargo_bin("seacloud").expect("seacloud binary")
}

#[test]
fn root_help_and_version_commands_work() {
    seacloud()
        .arg("--help")
        .assert()
        .success()
        .stdout(pred("SeaCloud CLI lets you manage your account"));

    seacloud()
        .arg("--version")
        .assert()
        .success()
        .stdout(pred("seacloud version"));

    seacloud()
        .arg("version")
        .assert()
        .success()
        .stdout(pred("dev"));
}

#[test]
fn dry_run_commands_do_not_need_network_or_credentials() {
    seacloud()
        .args(["--dry-run", "run", "seedance_2_0", "--param", "prompt=test"])
        .assert()
        .success()
        .stderr(pred("[dry-run] Would execute: POST <spec.api.endpoint>"));

    seacloud()
        .args([
            "--dry-run",
            "run",
            "gpt-image-2",
            "--param",
            "prompt=cat",
            "--output",
            "url",
        ])
        .assert()
        .success()
        .stderr(pred("/seacloud-cli-proxy-api/images/generations"));

    seacloud()
        .args(["--dry-run", "skills", "add", "cat", "--version", "1.0.0"])
        .assert()
        .success()
        .stderr(pred("[dry-run] Would install skill: cat"));
}

#[test]
fn auth_status_and_task_without_credentials_are_user_friendly() {
    let home = TempDir::new().unwrap();
    seacloud()
        .env("HOME", home.path())
        .env("SEACLOUD_NO_KEYCHAIN", "1")
        .arg("auth")
        .arg("status")
        .assert()
        .success()
        .stdout(pred("Not logged in."));

    seacloud()
        .env("HOME", home.path())
        .env("SEACLOUD_NO_KEYCHAIN", "1")
        .args(["task", "status", "task-1"])
        .assert()
        .failure()
        .stderr(pred("API key not set"));
}

#[test]
fn command_help_surfaces_subcommands() {
    for args in [
        vec!["auth", "--help"],
        vec!["models", "--help"],
        vec!["images", "--help"],
        vec!["skills", "--help"],
        vec!["task", "--help"],
        vec!["run", "--help"],
    ] {
        seacloud()
            .args(args)
            .assert()
            .success()
            .stdout(pred("Usage:"));
    }
}

fn pred(text: &'static str) -> impl predicates::Predicate<str> {
    predicates::str::contains(text)
}
