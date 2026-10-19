// Package apperr defines the errors that know how they should be reported to
// a client.
package apperr

import (
	"fmt"
	"net/http"
	"strings"
)

// AppError is an error that carries its own HTTP status, stable code, and
// client-safe detail. Error() is for operators and may include wrapped causes.
// Detail() is the only text a client ever sees.
type AppError interface {
	error
	Status() int
	Code() string
	Detail() string
}

// DependencyUnavailableError reports that a downstream dependency refused a
// connection or timed out. Err is the original driver error.
type DependencyUnavailableError struct {
	Dependency string
	Err        error
}

func (e *DependencyUnavailableError) Error() string {
	return fmt.Sprintf("dependency %s unavailable: %v", e.Dependency, e.Err)
}

func (e *DependencyUnavailableError) Unwrap() error { return e.Err }

func (e *DependencyUnavailableError) Status() int { return http.StatusServiceUnavailable }

func (e *DependencyUnavailableError) Code() string { return "DEPENDENCY_UNAVAILABLE" }

func (e *DependencyUnavailableError) Detail() string {
	return fmt.Sprintf("A required dependency is unavailable: %s.", e.Dependency)
}

// FieldError names one invalid input and why it is invalid.
type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// ValidationError collects every invalid field in a request.
type ValidationError struct {
	Fields []FieldError
}

func (e *ValidationError) Error() string {
	parts := make([]string, 0, len(e.Fields))
	for _, field := range e.Fields {
		parts = append(parts, field.Field+": "+field.Message)
	}
	return "validation failed: " + strings.Join(parts, "; ")
}

func (e *ValidationError) Status() int { return http.StatusBadRequest }

func (e *ValidationError) Code() string { return "VALIDATION_FAILED" }

func (e *ValidationError) Detail() string { return "The request is invalid." }
