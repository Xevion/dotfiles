//! Row bookkeeping for scrolling wrapped text. A `Paragraph` can only scroll
//! by `u16` rows and re-wraps everything it is given, so large transcripts
//! are windowed instead: this records how many rows each logical line takes
//! at a given width, letting a frame hand `Paragraph` only the lines that
//! are actually on screen.

use ratatui::text::Line;
use ratatui::widgets::{Paragraph, Wrap};

pub struct WrapIndex {
    /// Cumulative row count at the end of each line.
    ends: Vec<usize>,
}

impl WrapIndex {
    #[must_use]
    pub fn build(lines: &[Line<'_>], width: u16) -> Self {
        let mut total = 0;
        let ends = lines
            .iter()
            .map(|line| {
                let rows = Paragraph::new(line.clone())
                    .wrap(Wrap { trim: false })
                    .line_count(width);
                total += rows.max(1);
                total
            })
            .collect();
        Self { ends }
    }

    #[must_use]
    pub fn total_rows(&self) -> usize {
        self.ends.last().copied().unwrap_or(0)
    }

    /// The line holding display row `row`, and how many of that line's rows
    /// lie above it. Rows past the end land on the last line.
    #[must_use]
    pub fn locate(&self, row: usize) -> (usize, usize) {
        let Some(last) = self.ends.len().checked_sub(1) else {
            return (0, 0);
        };
        let line = self.ends.partition_point(|&end| end <= row).min(last);
        let start = if line == 0 { 0 } else { self.ends[line - 1] };
        (line, row.saturating_sub(start))
    }

    /// Rows taken by lines `from..to`.
    #[must_use]
    pub fn rows_between(&self, from: usize, to: usize) -> usize {
        let before = |i: usize| if i == 0 { 0 } else { self.ends[i - 1] };
        before(to.min(self.ends.len())) - before(from.min(self.ends.len()))
    }

    #[must_use]
    pub fn line_count(&self) -> usize {
        self.ends.len()
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use assert2::check;

    fn lines(texts: &[&str]) -> Vec<Line<'static>> {
        texts.iter().map(|t| Line::from((*t).to_string())).collect()
    }

    #[test]
    fn short_lines_take_one_row_each() {
        let index = WrapIndex::build(&lines(&["a", "b", "c"]), 10);
        check!(index.total_rows() == 3);
    }

    #[test]
    fn empty_lines_still_take_a_row() {
        let index = WrapIndex::build(&lines(&["a", "", "b"]), 10);
        check!(index.total_rows() == 3);
    }

    #[test]
    fn long_lines_wrap_to_several_rows() {
        let index = WrapIndex::build(&lines(&["aaaa bbbb cccc dddd", "x"]), 10);
        check!(index.total_rows() == 3);
    }

    #[test]
    fn locate_maps_rows_into_wrapped_lines() {
        let index = WrapIndex::build(&lines(&["aaaa bbbb cccc dddd", "x"]), 10);
        check!(index.locate(0) == (0, 0));
        check!(index.locate(1) == (0, 1));
        check!(index.locate(2) == (1, 0));
    }

    #[test]
    fn locate_past_the_end_clamps_to_the_last_line() {
        let index = WrapIndex::build(&lines(&["a", "b"]), 10);
        check!(index.locate(99).0 == 1);
    }

    #[test]
    fn locate_in_nothing_is_the_origin() {
        let index = WrapIndex::build(&[], 10);
        check!(index.locate(5) == (0, 0));
        check!(index.total_rows() == 0);
    }

    #[test]
    fn rows_between_sums_the_span() {
        let index = WrapIndex::build(&lines(&["aaaa bbbb cccc dddd", "x", "y"]), 10);
        check!(index.rows_between(0, 1) == 2);
        check!(index.rows_between(1, 3) == 2);
        check!(index.rows_between(0, 3) == 4);
    }
}
