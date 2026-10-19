package orders

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"net"
	"syscall"
	"uuid"

	"github.com/rwaterman/error-handling-examples/go/apperr"
)

const storeDependency = "order-store"

// Repository persists orders in a Store. It translates driver connection and
// timeout failures into *apperr.DependencyUnavailableError. Every other error
// is returned wrapped with context, so errors.Is still finds the original.
type Repository struct {
	store Store
}

// NewRepository returns a Repository backed by store.
func NewRepository(store Store) *Repository {
	return &Repository{store: store}
}

// Find returns the order with the given ID. found is false when no such order
// exists, which is not an error at this layer.
func (r *Repository) Find(ctx context.Context, id uuid.UUID) (order Order, found bool, err error) {
	record, found, err := r.store.Get(ctx, id.String())
	if err != nil {
		return Order{}, false, classify(fmt.Errorf("get order %s: %w", id, err))
	}
	if !found {
		return Order{}, false, nil
	}
	if err := json.Unmarshal(record, &order); err != nil {
		return Order{}, false, fmt.Errorf("decode order %s: %w", id, err)
	}
	return order, true, nil
}

// Save stores the order under its ID, replacing any previous version.
func (r *Repository) Save(ctx context.Context, order Order) error {
	record, err := json.Marshal(order)
	if err != nil {
		return fmt.Errorf("encode order %s: %w", order.ID, err)
	}
	if err := r.store.Put(ctx, order.ID.String(), record); err != nil {
		return classify(fmt.Errorf("put order %s: %w", order.ID, err))
	}
	return nil
}

func classify(err error) error {
	if !isUnavailable(err) {
		return err
	}
	return &apperr.DependencyUnavailableError{Dependency: storeDependency, Err: err}
}

func isUnavailable(err error) bool {
	if errors.Is(err, syscall.ECONNREFUSED) || errors.Is(err, syscall.ECONNRESET) {
		return true
	}
	timeout, ok := errors.AsType[net.Error](err)
	return ok && timeout.Timeout()
}
