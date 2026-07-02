use std::process::exit;

fn main() {
    let mut args = std::env::args().skip(1);
    let code = match args.next().as_deref() {
        Some("hook") => guard::hook::main(),
        Some("run") => {
            let pipeline = args.next().unwrap_or_default();
            guard::run::main(&pipeline)
        }
        other => {
            eprintln!(
                "guard: expected subcommand `hook` or `run`, got {other:?}\n\
                 usage: guard hook   (reads PreToolUse JSON on stdin)\n\
                        guard run '<pipeline>'"
            );
            2
        }
    };
    exit(code);
}
