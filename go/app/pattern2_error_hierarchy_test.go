package app_test

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"uuid"

	"github.com/rwaterman/error-handling-examples/go/apperr"
	"github.com/rwaterman/error-handling-examples/go/orders"
)

func TestDependencyUnavailableCarriesItsStatusAndCode(t *testing.T) {
	dependencyErr := &apperr.DependencyUnavailableError{Dependency: "order-store", Err: refusedConnection(secret)}

	var appErr apperr.AppError = dependencyErr
	if appErr.Status() != http.StatusServiceUnavailable {
		t.Errorf("Status() = %d, want 503", appErr.Status())
	}
	if appErr.Code() != "DEPENDENCY_UNAVAILABLE" {
		t.Errorf("Code() = %q, want DEPENDENCY_UNAVAILABLE", appErr.Code())
	}
	if dependencyErr.Dependency != "order-store" {
		t.Errorf("Dependency = %q, want order-store", dependencyErr.Dependency)
	}
	if appErr.Detail() != "A required dependency is unavailable: order-store." {
		t.Errorf("Detail() = %q", appErr.Detail())
	}
	if strings.Contains(appErr.Detail(), secret) || !strings.Contains(appErr.Error(), secret) {
		t.Errorf("want the cause in Error() for operators and out of Detail() for clients; Error() = %q, Detail() = %q",
			appErr.Error(), appErr.Detail())
	}
}

func TestRepositoryTranslatesDriverErrorsAndKeepsTheCause(t *testing.T) {
	tests := []struct {
		name        string
		driverError error
	}{
		{"connection refused", refusedConnection("db-01:5432")},
		{"read timeout", readTimeout()},
		{"context deadline", context.DeadlineExceeded},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repository := orders.NewRepository(failingStore{err: test.driverError})

			_, _, err := repository.Find(t.Context(), uuid.New())

			dependencyErr, ok := errors.AsType[*apperr.DependencyUnavailableError](err)
			if !ok {
				t.Fatalf("Find returned %v, want a *apperr.DependencyUnavailableError", err)
			}
			if !errors.Is(dependencyErr, test.driverError) {
				t.Errorf("the driver error is not in the cause chain of %v", dependencyErr)
			}
		})
	}
}

func TestRepositoryLetsUnexpectedErrorsThroughUntouched(t *testing.T) {
	bug := errors.New("decode record: missing customerEmail")
	repository := orders.NewRepository(failingStore{err: bug})

	_, _, err := repository.Find(t.Context(), uuid.New())

	if !errors.Is(err, bug) {
		t.Fatalf("Find returned %v, want an error that wraps the original", err)
	}
	if _, ok := errors.AsType[*apperr.DependencyUnavailableError](err); ok {
		t.Fatalf("Find relabeled a bug as an outage: %v", err)
	}
}
