use std::process::exit;

fn main() {
    let mut args = std::env::args().skip(1);
    let code = match args.next().as_deref() {
        None | Some("hook") => guard::dispatch::main(),
        Some("run") => {
            let pipeline = args.next().unwrap_or_default();
            guard::run::main(&pipeline)
        }
        Some("check") => {
            let path = args.next().unwrap_or_default();
            guard::check::main(&path)
        }
        other => {
            eprintln!(
                "guard: unrecognized argument {other:?}\n\
                 usage: guard               (reads hook JSON on stdin, dispatches by event+tool)\n\
                        guard hook          (alias for the above)\n\
                        guard run '<pipeline>'\n\
                        guard check <path>  (dry-run comment-lint over a file or directory)"
            );
            2
        }
    };
    exit(code);
}
