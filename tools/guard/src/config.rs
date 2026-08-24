//! Per-project settings read from `.git/info/guard.toml`. Missing, unreadable,
//! or malformed config always falls back to defaults - a broken config file
//! must never block a hook.

use globset::{Glob, GlobSetBuilder};
use serde::Deserialize;
use std::path::Path;

#[derive(Debug, Clone, Default, Deserialize)]
#[serde(default)]
pub struct Config {
    pub hooks: HooksConfig,
    #[serde(rename = "comment-lint")]
    pub comment_lint: CommentLintConfig,
}

#[derive(Debug, Clone, Default, Deserialize)]
#[serde(default)]
pub struct HooksConfig {
    pub disabled: Vec<String>,
}

#[derive(Debug, Clone, Deserialize)]
#[serde(default)]
pub struct CommentLintConfig {
    pub enabled: bool,
    pub ignore: Vec<String>,
}

impl Default for CommentLintConfig {
    fn default() -> Self {
        Self {
            enabled: true,
            ignore: Vec::new(),
        }
    }
}

impl Config {
    /// Load `.git/info/guard.toml` for the repo containing `start`. Any
    /// failure along the way (no repo, missing file, unreadable, malformed
    /// TOML) yields `Config::default()`.
    pub fn load(start: &Path) -> Self {
        crate::git::find_repo(start)
            .and_then(|repo| std::fs::read_to_string(repo.git_dir.join("info/guard.toml")).ok())
            .and_then(|text| toml::from_str(&text).ok())
            .unwrap_or_default()
    }

    /// Whether `hook_id` should run for `file` (when applicable) under this
    /// project's config.
    pub fn hook_enabled(&self, hook_id: &str, file: Option<&str>) -> bool {
        if self.hooks.disabled.iter().any(|d| d == hook_id) {
            return false;
        }
        match hook_id {
            "comment-lint" => self.comment_lint.enabled && !self.comment_lint.is_ignored(file),
            _ => true,
        }
    }
}

impl CommentLintConfig {
    fn is_ignored(&self, file: Option<&str>) -> bool {
        if self.ignore.is_empty() {
            return false;
        }
        let Some(file) = file else { return false };
        let mut builder = GlobSetBuilder::new();
        for pattern in &self.ignore {
            if let Ok(glob) = Glob::new(pattern) {
                builder.add(glob);
            }
        }
        builder.build().is_ok_and(|set| set.is_match(file))
    }
}

#[cfg(test)]
mod tests;
