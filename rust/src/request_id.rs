use std::fmt;

use axum::http::{HeaderMap, HeaderName, HeaderValue};
use uuid::Uuid;

pub const X_REQUEST_ID: HeaderName = HeaderName::from_static("x-request-id");

const MAX_LENGTH: usize = 255;

#[derive(Clone, Debug, PartialEq, Eq)]
pub struct RequestId(String);

impl RequestId {
    pub fn from_headers(headers: &HeaderMap) -> Self {
        headers
            .get(X_REQUEST_ID)
            .and_then(|value| value.to_str().ok())
            .filter(|candidate| is_trusted(candidate))
            .map_or_else(Self::generate, |trusted| Self(trusted.to_owned()))
    }

    pub fn as_str(&self) -> &str {
        &self.0
    }

    pub fn header_value(&self) -> HeaderValue {
        HeaderValue::from_str(&self.0)
            .expect("request ids contain only ASCII letters, digits, '_', '=' and '-'")
    }

    fn generate() -> Self {
        Self(Uuid::new_v4().to_string())
    }
}

impl fmt::Display for RequestId {
    fn fmt(&self, formatter: &mut fmt::Formatter<'_>) -> fmt::Result {
        formatter.write_str(&self.0)
    }
}

fn is_trusted(candidate: &str) -> bool {
    (1..=MAX_LENGTH).contains(&candidate.len())
        && candidate
            .bytes()
            .all(|byte| byte.is_ascii_alphanumeric() || matches!(byte, b'_' | b'=' | b'-'))
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn accepts_only_bounded_url_safe_ids() {
        assert!(is_trusted("req-123_abc="));
        assert!(is_trusted(&"a".repeat(MAX_LENGTH)));
        assert!(!is_trusted(""));
        assert!(!is_trusted("has space"));
        assert!(!is_trusted("line\nbreak"));
        assert!(!is_trusted(&"a".repeat(MAX_LENGTH + 1)));
    }
}
