// Package app wires the orders API: routes, the central error handler, panic
// recovery, and request IDs.
package app

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/rwaterman/error-handling-examples/go/orders"
)

const maxBodyBytes = 1 << 20

type server struct {
	service *orders.Service
	logger  *slog.Logger
}

// New returns the HTTP handler for the orders API. logger receives exactly one
// ERROR record for each request that ends in a 5xx response.
func New(service *orders.Service, logger *slog.Logger) http.Handler {
	s := &server{service: service, logger: logger}
	mux := http.NewServeMux()
	mux.Handle("POST /orders", s.handle(s.createOrder))
	mux.Handle("GET /orders/{orderId}", s.handle(s.getOrder))
	mux.Handle("POST /orders/{orderId}/process", s.handle(s.processOrder))
	mux.Handle("/", s.handle(routeNotFound))
	return withRequestID(s.recoverPanics(mux))
}

func (s *server) createOrder(w http.ResponseWriter, r *http.Request) error {
	request, err := orders.DecodeCreateOrderRequest(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	if err != nil {
		return err
	}
	order, err := s.service.Create(r.Context(), request)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusCreated, order)
}

func (s *server) getOrder(w http.ResponseWriter, r *http.Request) error {
	id, err := orders.ParseOrderID(r.PathValue("orderId"))
	if err != nil {
		return err
	}
	order, err := s.service.Get(r.Context(), id)
	if err != nil {
		return writeExpectedOutcome(w, r, err)
	}
	return writeJSON(w, http.StatusOK, order)
}

func (s *server) processOrder(w http.ResponseWriter, r *http.Request) error {
	id, err := orders.ParseOrderID(r.PathValue("orderId"))
	if err != nil {
		return err
	}
	order, err := s.service.Process(r.Context(), id)
	if err != nil {
		return writeExpectedOutcome(w, r, err)
	}
	return writeJSON(w, http.StatusOK, order)
}

// writeExpectedOutcome writes the response for an expected order outcome and
// returns nil. Any other error is returned unchanged for the central handler.
func writeExpectedOutcome(w http.ResponseWriter, r *http.Request, err error) error {
	if notFound, ok := errors.AsType[orders.NotFoundError](err); ok {
		detail := fmt.Sprintf("Order %s does not exist.", notFound.OrderID)
		return writeProblem(w, r, http.StatusNotFound, "ORDER_NOT_FOUND", detail, nil)
	}
	if processed, ok := errors.AsType[orders.AlreadyProcessedError](err); ok {
		detail := fmt.Sprintf("Order %s was already processed.", processed.OrderID)
		return writeProblem(w, r, http.StatusConflict, "ORDER_ALREADY_PROCESSED", detail, nil)
	}
	return err
}

func routeNotFound(w http.ResponseWriter, r *http.Request) error {
	return writeProblem(w, r, http.StatusNotFound, "ROUTE_NOT_FOUND", http.StatusText(http.StatusNotFound), nil)
}
