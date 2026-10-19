package app_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"testing"
	"uuid"

	"github.com/rwaterman/error-handling-examples/go/apperr"
	"github.com/rwaterman/error-handling-examples/go/orders"
)

// findOrderSwallowingErrors is Go's version of catch-everything: it checks
// err only to throw it away.
func findOrderSwallowingErrors(ctx context.Context, repository *orders.Repository, id uuid.UUID) (orders.Order, bool) {
	order, found, err := repository.Find(ctx, id)
	if err != nil {
		return orders.Order{}, false
	}
	return order, found
}

func captureDefaultLogger(t *testing.T) *bytes.Buffer {
	t.Helper()
	logs := &bytes.Buffer{}
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(logs, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })
	return logs
}

func TestSwallowingMakesAnOutageIndistinguishableFromAMissingOrder(t *testing.T) {
	logs := captureDefaultLogger(t)
	healthy := orders.NewRepository(orders.NewMemoryStore())
	down := orders.NewRepository(failingStore{err: refusedConnection("db-01:5432")})

	_, missingFound := findOrderSwallowingErrors(t.Context(), healthy, uuid.New())
	_, outageFound := findOrderSwallowingErrors(t.Context(), down, uuid.New())

	if missingFound || outageFound {
		t.Fatalf("found = %t (missing) and %t (outage), want false for both", missingFound, outageFound)
	}
	if logs.Len() != 0 {
		t.Fatalf("want nothing logged, got %s", logs.String())
	}
}

func TestLettingItPropagateKeepsTheOutageVisible(t *testing.T) {
	down := orders.NewRepository(failingStore{err: refusedConnection("db-01:5432")})

	_, _, err := down.Find(t.Context(), uuid.New())

	if _, ok := errors.AsType[*apperr.DependencyUnavailableError](err); !ok {
		t.Fatalf("Find returned %v, want a *apperr.DependencyUnavailableError", err)
	}
}
