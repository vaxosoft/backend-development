//! Build script: runs before the crate compiles and can export values to it with `cargo:rustc-env`.

fn main() {
    // TARGET and PROFILE are provided by cargo to build scripts
    for key in ["TARGET", "PROFILE"] {
        let value = std::env::var(key).unwrap_or_else(|_| "unknown".to_string());
        println!("cargo:rustc-env=BUILD_{key}={value}");
    }
    // Optional, supplied by CI: `GIT_SHA=$(git rev-parse HEAD) cargo build --release`
    println!("cargo:rerun-if-env-changed=GIT_SHA");
    println!("cargo:rerun-if-changed=build.rs");
}
