//! The result of one hook handler: an exit code plus enough context for
//! `logging` to record which rules fired and whether the handler abstained
//! rather than finding nothing.

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Outcome {
    pub exit_code: i32,
    pub rules: Vec<String>,
    pub abstained: bool,
}

impl Outcome {
    pub fn from_exit_code(exit_code: i32) -> Self {
        Self {
            exit_code,
            rules: Vec::new(),
            abstained: false,
        }
    }

    pub fn allow() -> Self {
        Self::from_exit_code(0)
    }

    pub fn abstain() -> Self {
        Self {
            exit_code: 0,
            rules: Vec::new(),
            abstained: true,
        }
    }

    pub fn block(rules: Vec<String>) -> Self {
        Self {
            exit_code: 2,
            rules,
            abstained: false,
        }
    }

    pub fn nudge(rules: Vec<String>) -> Self {
        Self {
            exit_code: 0,
            rules,
            abstained: false,
        }
    }
}

#[cfg(test)]
mod tests;
