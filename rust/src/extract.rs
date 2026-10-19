use axum::Json;
use axum::extract::rejection::JsonRejection;
use axum::extract::{FromRequest, FromRequestParts, Path, Request};
use axum::http::request::Parts;
use serde::de::DeserializeOwned;
use serde_path_to_error::Segment;
use uuid::Uuid;
use validator::{Validate, ValidationErrors, ValidationErrorsKind};

use crate::errors::{AppError, FieldError};

/// A JSON body that is well-formed, has the expected shape, and passes its
/// `Validate` rules. Handlers that take it never see invalid input.
pub struct ValidJson<T>(pub T);

impl<S, T> FromRequest<S> for ValidJson<T>
where
    S: Send + Sync,
    T: DeserializeOwned + Validate,
{
    type Rejection = AppError;

    async fn from_request(request: Request, state: &S) -> Result<Self, Self::Rejection> {
        let Json(document) = Json::<serde_json::Value>::from_request(request, state)
            .await
            .map_err(body_error)?;
        let value: T = serde_path_to_error::deserialize(document).map_err(shape_error)?;
        value.validate().map_err(rule_errors)?;
        Ok(Self(value))
    }
}

/// The `orderId` path segment parsed as a UUID.
pub struct OrderIdParam(pub Uuid);

impl<S> FromRequestParts<S> for OrderIdParam
where
    S: Send + Sync,
{
    type Rejection = AppError;

    async fn from_request_parts(parts: &mut Parts, state: &S) -> Result<Self, Self::Rejection> {
        let Path(order_id) = Path::<Uuid>::from_request_parts(parts, state)
            .await
            .map_err(|rejection| invalid("orderId", rejection.body_text()))?;
        Ok(Self(order_id))
    }
}

fn invalid(field: &str, message: String) -> AppError {
    AppError::Validation(vec![FieldError::new(field, message)])
}

fn body_error(rejection: JsonRejection) -> AppError {
    invalid("body", rejection.body_text())
}

fn shape_error(error: serde_path_to_error::Error<serde_json::Error>) -> AppError {
    let field = error
        .path()
        .iter()
        .filter_map(segment_name)
        .collect::<Vec<_>>()
        .join(".");
    let field = if field.is_empty() {
        "body".to_owned()
    } else {
        field
    };
    invalid(&field, error.into_inner().to_string())
}

fn segment_name(segment: &Segment) -> Option<String> {
    match segment {
        Segment::Seq { index } => Some(index.to_string()),
        Segment::Map { key } => Some(key.clone()),
        Segment::Enum { variant } => Some(variant.clone()),
        Segment::Unknown => None,
    }
}

fn rule_errors(errors: ValidationErrors) -> AppError {
    let mut fields = Vec::new();
    collect_field_errors(&errors, None, &mut fields);
    fields.sort_by(|left, right| left.field.cmp(&right.field));
    AppError::Validation(fields)
}

fn collect_field_errors(
    errors: &ValidationErrors,
    prefix: Option<&str>,
    out: &mut Vec<FieldError>,
) {
    for (name, kind) in errors.errors() {
        let path = match prefix {
            Some(prefix) => format!("{prefix}.{}", camel_case(name)),
            None => camel_case(name),
        };
        match kind {
            ValidationErrorsKind::Field(failures) => out.extend(
                failures
                    .iter()
                    .map(|failure| FieldError::new(&path, failure.to_string())),
            ),
            ValidationErrorsKind::Struct(nested) => collect_field_errors(nested, Some(&path), out),
            ValidationErrorsKind::List(entries) => {
                for (index, nested) in entries {
                    collect_field_errors(nested, Some(&format!("{path}.{index}")), out);
                }
            }
        }
    }
}

/// `validator` reports Rust field names and ignores `#[serde(rename_all)]`.
fn camel_case(snake: &str) -> String {
    let mut words = snake.split('_');
    let first = words.next().unwrap_or_default().to_owned();
    words.fold(first, |mut camel, word| {
        let mut chars = word.chars();
        if let Some(initial) = chars.next() {
            camel.extend(initial.to_uppercase());
            camel.push_str(chars.as_str());
        }
        camel
    })
}

#[cfg(test)]
mod tests {
    use super::camel_case;

    #[test]
    fn converts_rust_field_names_to_wire_names() {
        assert_eq!(camel_case("customer_email"), "customerEmail");
        assert_eq!(camel_case("items"), "items");
    }
}
