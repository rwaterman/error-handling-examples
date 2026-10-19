use tracing::Subscriber;
use tracing_subscriber::filter::LevelFilter;
use tracing_subscriber::fmt::MakeWriter;

/// JSON lines with event fields at the top level, shared by `main` and the tests.
pub fn json_subscriber<W>(writer: W, max_level: LevelFilter) -> impl Subscriber + Send + Sync
where
    W: for<'writer> MakeWriter<'writer> + Send + Sync + 'static,
{
    tracing_subscriber::fmt()
        .json()
        .flatten_event(true)
        .with_current_span(false)
        .with_span_list(false)
        .with_max_level(max_level)
        .with_writer(writer)
        .finish()
}
