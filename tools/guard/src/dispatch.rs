//! Entry point for the hook program itself: reads the hook JSON from stdin
//! once, routes it by event and tool, and logs the outcome. `guard hook` is
//! an alias into this same path.

use crate::payload::Payload;
use std::io::Read;
use std::time::{Duration, Instant};

pub fn main() -> i32 {
    let mut buf = String::new();
    if std::io::stdin().read_to_string(&mut buf).is_err() {
        return 0;
    }
    let payload = match serde_json::from_str::<Payload>(&buf) {
        Ok(payload) => payload,
        Err(_) => {
            let unparsed = Payload {
                hook_event_name: "<unparsed>".to_string(),
                ..Payload::default()
            };
            crate::logging::record(&unparsed, 0, Duration::ZERO);
            return 0;
        }
    };
    let started = Instant::now();
    let code = route(&payload);
    crate::logging::record(&payload, code, started.elapsed());
    code
}

#[derive(Debug, PartialEq, Eq)]
enum Handler {
    PreToolUseBash,
    PostToolUseEdit,
    None,
}

/// An absent event name routes on tool alone, matching the behavior from
/// before the dispatch layer existed.
fn classify(event: &str, tool: &str) -> Handler {
    match (event, tool) {
        ("PreToolUse" | "", "Bash") => Handler::PreToolUseBash,
        ("PostToolUse", "Write" | "Edit") => Handler::PostToolUseEdit,
        _ => Handler::None,
    }
}

fn route(payload: &Payload) -> i32 {
    match classify(&payload.hook_event_name, &payload.tool_name) {
        Handler::PreToolUseBash => crate::hook::main(payload),
        Handler::PostToolUseEdit => crate::post_edit::main(payload),
        Handler::None => 0,
    }
}

#[cfg(test)]
mod tests;
