package orders

import (
	"context"
	"sync"
	"time"
	"uuid"
)

// Service implements the order use cases.
type Service struct {
	repository *Repository
	processMu  sync.Mutex
}

// NewService returns a Service that stores orders through repository.
func NewService(repository *Repository) *Service {
	return &Service{repository: repository}
}

// Create stores a new pending order.
func (s *Service) Create(ctx context.Context, request CreateOrderRequest) (Order, error) {
	order := Order{
		ID:            uuid.New(),
		CustomerEmail: request.CustomerEmail,
		Items:         request.Items,
		Status:        StatusPending,
		CreatedAt:     time.Now().UTC(),
	}
	if err := s.repository.Save(ctx, order); err != nil {
		return Order{}, err
	}
	return order, nil
}

// Get returns the order with the given ID. The expected failure is
// NotFoundError. Any other error is unexpected.
func (s *Service) Get(ctx context.Context, id uuid.UUID) (Order, error) {
	order, found, err := s.repository.Find(ctx, id)
	if err != nil {
		return Order{}, err
	}
	if !found {
		return Order{}, NotFoundError{OrderID: id}
	}
	return order, nil
}

// Process marks a pending order as processed. The expected failures are
// NotFoundError and AlreadyProcessedError. Any other error is unexpected.
func (s *Service) Process(ctx context.Context, id uuid.UUID) (Order, error) {
	s.processMu.Lock()
	defer s.processMu.Unlock()

	order, err := s.Get(ctx, id)
	if err != nil {
		return Order{}, err
	}
	if order.Status == StatusProcessed {
		return Order{}, AlreadyProcessedError{OrderID: id}
	}
	processedAt := time.Now().UTC()
	order.Status = StatusProcessed
	order.ProcessedAt = &processedAt
	if err := s.repository.Save(ctx, order); err != nil {
		return Order{}, err
	}
	return order, nil
}
