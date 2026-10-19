// Package orders holds the order domain: validation, storage, and the service
// that the HTTP layer calls.
package orders

import (
	"fmt"
	"time"
	"uuid"
)

// Status is the lifecycle state of an order.
type Status string

const (
	StatusPending   Status = "pending"
	StatusProcessed Status = "processed"
)

// Item is one line of an order.
type Item struct {
	SKU      string `json:"sku"`
	Quantity int    `json:"quantity"`
}

// Order is the stored and returned representation of an order.
type Order struct {
	ID            uuid.UUID  `json:"id"`
	CustomerEmail string     `json:"customerEmail"`
	Items         []Item     `json:"items"`
	Status        Status     `json:"status"`
	CreatedAt     time.Time  `json:"createdAt"`
	ProcessedAt   *time.Time `json:"processedAt"`
}

// NotFoundError is the expected outcome when no order has the requested ID.
type NotFoundError struct {
	OrderID uuid.UUID
}

func (e NotFoundError) Error() string {
	return fmt.Sprintf("order %s not found", e.OrderID)
}

// AlreadyProcessedError is the expected outcome when an order is processed a
// second time.
type AlreadyProcessedError struct {
	OrderID uuid.UUID
}

func (e AlreadyProcessedError) Error() string {
	return fmt.Sprintf("order %s already processed", e.OrderID)
}
