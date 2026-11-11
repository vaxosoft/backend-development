//! What the binary knows about how it was built. Compare `cargo run` with `cargo run --release`.

use m14_packaging_and_running::BUILD;

fn main() {
    println!("name:    {}", BUILD.name);
    println!("version: {}", BUILD.version);
    println!("profile: {}", BUILD.profile);
    println!("target:  {}", BUILD.target);
    println!("commit:  {}", BUILD.git_sha.unwrap_or("unknown"));
    println!("debug assertions: {}", cfg!(debug_assertions));
    println!("pointer width: {} bits", usize::BITS);
}
