use snafu::Snafu;

#[derive(Debug, Snafu)]
#[snafu(visibility(pub))]
pub enum AppError {
    #[snafu(display("failed to draw a frame: {source}"))]
    Draw { source: std::io::Error },
    #[snafu(display("failed to start watching {path}: {source}"))]
    Watch { path: String, source: notify::Error },
}
