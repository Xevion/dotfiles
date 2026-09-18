//! Short display form for absolute paths (`~/projects/foo` instead of the
//! raw encoded project directory or a full absolute path).

use std::path::Path;

pub trait ShortenExt {
    fn shorten(&self, home: &Path) -> String;
}

impl ShortenExt for Path {
    fn shorten(&self, home: &Path) -> String {
        match self.strip_prefix(home) {
            Ok(rel) if rel.as_os_str().is_empty() => "~".to_string(),
            Ok(rel) => format!("~/{}", rel.display()),
            Err(_) => self.display().to_string(),
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use assert2::check;

    #[test]
    fn shortens_paths_under_home() {
        let home = Path::new("/home/xevion");
        check!(Path::new("/home/xevion/projects/foo").shorten(home) == "~/projects/foo");
    }

    #[test]
    fn home_itself_shortens_to_tilde() {
        let home = Path::new("/home/xevion");
        check!(Path::new("/home/xevion").shorten(home) == "~");
    }

    #[test]
    fn leaves_unrelated_paths_untouched() {
        let home = Path::new("/home/xevion");
        check!(Path::new("/var/tmp/other").shorten(home) == "/var/tmp/other");
    }
}
