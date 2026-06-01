fn main() {
    if let Err(err) = seacloud_cli::cmd::execute() {
        eprintln!("Error: {err}");
        std::process::exit(1);
    }
}
