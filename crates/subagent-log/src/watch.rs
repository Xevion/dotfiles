//! One recursive watch on `~/.claude/projects/`: inotify cost is
//! per-directory, so this beats one watch per `subagents/` dir.

use std::path::Path;
use std::time::Duration;

use crossbeam_channel::Sender;
use notify::{EventKind, RecommendedWatcher, RecursiveMode};
use notify_debouncer_full::{DebounceEventResult, Debouncer, RecommendedCache, new_debouncer};

pub struct Watch {
    // Kept only to hold the debouncer's background thread alive.
    _debouncer: Debouncer<RecommendedWatcher, RecommendedCache>,
}

impl Watch {
    pub fn spawn(root: &Path, tx: Sender<()>) -> notify::Result<Self> {
        let mut debouncer = new_debouncer(
            Duration::from_millis(150),
            None,
            move |result: DebounceEventResult| {
                if let Ok(events) = result
                    && events.iter().any(Self::is_relevant)
                {
                    let _ = tx.send(());
                }
            },
        )?;
        debouncer.watch(root, RecursiveMode::Recursive)?;
        Ok(Self {
            _debouncer: debouncer,
        })
    }

    fn is_relevant(event: &notify_debouncer_full::DebouncedEvent) -> bool {
        matches!(
            event.kind,
            EventKind::Create(_) | EventKind::Modify(_) | EventKind::Remove(_)
        )
    }
}
