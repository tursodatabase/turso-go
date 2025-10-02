use std::process::Command;

fn main() {
    // Get git SHA for version tracking
    let output = Command::new("git")
        .args(["rev-parse", "--short=8", "HEAD"])
        .output()
        .expect("Failed to get git SHA");

    let git_sha = String::from_utf8(output.stdout)
        .expect("Invalid UTF-8")
        .trim()
        .to_string();

    // Rerun if git HEAD changes
    println!("cargo:rerun-if-changed=.git/HEAD");
    println!("cargo:rerun-if-changed=.git/index");

    std::fs::write("VERSION", &git_sha).expect("Failed to write VERSION file");
}
