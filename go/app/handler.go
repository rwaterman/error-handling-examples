package app

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/rwaterman/error-handling-examples/go/apperr"
)

const (
	internalErrorCode   = "INTERNAL_ERROR"
	internalErrorDetail = "An unexpected error occurred."
)

// handlerFunc is an HTTP handler that returns the errors it does not handle
// itself. handle is the one place those errors become responses.
type handlerFunc func(w http.ResponseWriter, r *http.Request) error

func (s *server) handle(fn handlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := fn(w, r); err != nil {
			s.fail(w, r, err)
		}
	})
}

// fail is the central error handler. An AppError supplies its own status,
// code, and detail. Anything else is a bug and gets the generic 500. Every 5xx
// is logged once with the full error. Nothing below 500 is logged.
func (s *server) fail(w http.ResponseWriter, r *http.Request, err error) {
	appErr, ok := errors.AsType[apperr.AppError](err)
	if !ok {
		s.logFailure(r, internalErrorCode, err)
		s.respondProblem(w, r, http.StatusInternalServerError, internalErrorCode, internalErrorDetail, nil)
		return
	}
	if appErr.Status() >= http.StatusInternalServerError {
		s.logFailure(r, appErr.Code(), err)
	}
	var fields []apperr.FieldError
	if validation, ok := errors.AsType[*apperr.ValidationError](err); ok {
		fields = validation.Fields
	}
	s.respondProblem(w, r, appErr.Status(), appErr.Code(), appErr.Detail(), fields)
}

// recoverPanics turns a panic into the same generic 500 and ERROR record as
// an unexpected error, with the stack attached.
func (s *server) recoverPanics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			recovered := recover()
			if recovered == nil {
				return
			}
			if recovered == http.ErrAbortHandler {
				panic(recovered)
			}
			err := fmt.Errorf("panic: %v", recovered)
			s.logFailure(r, internalErrorCode, err, slog.String("stack", string(debug.Stack())))
			s.respondProblem(w, r, http.StatusInternalServerError, internalErrorCode, internalErrorDetail, nil)
		}()
		next.ServeHTTP(w, r)
	})
}

func (s *server) respondProblem(w http.ResponseWriter, r *http.Request, status int, code, detail string, fields []apperr.FieldError) {
	if err := writeProblem(w, r, status, code, detail, fields); err != nil {
		s.logFailure(r, internalErrorCode, err)
		w.WriteHeader(http.StatusInternalServerError)
	}
}

func (s *server) logFailure(r *http.Request, code string, err error, extra ...slog.Attr) {
	attrs := []slog.Attr{
		slog.String("request_id", requestIDFrom(r.Context())),
		slog.String("code", code),
		slog.String("path", r.URL.Path),
		slog.Any("error", err),
	}
	s.logger.LogAttrs(r.Context(), slog.LevelError, "request failed", append(attrs, extra...)...)
}
