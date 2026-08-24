use std::process::exit;

fn main() {
    let mut args = std::env::args().skip(1);
    let code = match args.next().as_deref() {
        None | Some("hook") => guard::dispatch::main(),
        Some("run") => {
            let pipeline = args.next().unwrap_or_default();
            guard::run::main(&pipeline)
        }
        other => {
            eprintln!(
                "guard: unrecognized argument {other:?}\n\
                 usage: guard               (reads hook JSON on stdin, dispatches by event+tool)\n\
                        guard hook          (alias for the above)\n\
                        guard run '<pipeline>'"
            );
            2
        }
    };
    exit(code);
}
