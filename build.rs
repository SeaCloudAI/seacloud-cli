use std::env;

fn main() {
    let version = env::var("SEACLOUD_BUILD_VERSION")
        .or_else(|_| env::var("VERSION"))
        .unwrap_or_else(|_| "dev".to_string());
    println!("cargo:rustc-env=SEACLOUD_BUILD_VERSION={version}");
    println!("cargo:rerun-if-env-changed=SEACLOUD_BUILD_VERSION");
    println!("cargo:rerun-if-env-changed=VERSION");

    for name in [
        "SEACLOUD_BASE_URL",
        "SEACLOUD_MODELS_URL",
        "SEACLOUD_GENERATION_URL",
        "SEACLOUD_SKILLHUB_URL",
        "SEACLOUD_FOLKOS_PROXY_URL",
    ] {
        println!("cargo:rerun-if-env-changed={name}");
        if let Ok(value) = env::var(name) {
            println!("cargo:rustc-env={name}={value}");
        }
    }
}
