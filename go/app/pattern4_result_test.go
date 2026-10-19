package app_test

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
	"uuid"

	"github.com/rwaterman/error-handling-examples/go/orders"
)

func newService() *orders.Service {
	return orders.NewService(orders.NewRepository(orders.NewMemoryStore()))
}

func TestGetUnknownOrderReturnsNotFoundValue(t *testing.T) {
	orderID := uuid.New()

	_, err := newService().Get(t.Context(), orderID)

	if !errors.Is(err, orders.NotFoundError{OrderID: orderID}) {
		t.Fatalf("Get returned %v, want NotFoundError for %s", err, orderID)
	}
}

func TestProcessingTwiceReturnsAlreadyProcessedValue(t *testing.T) {
	service := newService()
	request := orders.CreateOrderRequest{CustomerEmail: "ada@example.com", Items: []orders.Item{{SKU: "BOOK-1", Quantity: 2}}}
	order, err := service.Create(t.Context(), request)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := service.Process(t.Context(), order.ID); err != nil {
		t.Fatalf("first Process: %v", err)
	}

	_, err = service.Process(t.Context(), order.ID)

	if !errors.Is(err, orders.AlreadyProcessedError{OrderID: order.ID}) {
		t.Fatalf("second Process returned %v, want AlreadyProcessedError for %s", err, order.ID)
	}
}

func createOrder(t *testing.T, testApp *testApp) uuid.UUID {
	t.Helper()
	response := testApp.postJSON(t, "/orders", validOrder())
	assertStatus(t, response, http.StatusCreated)
	return decodeJSON[orders.Order](t, response).ID
}

func TestUnknownOrderMapsTo404(t *testing.T) {
	orderID := uuid.New()

	response := newTestApp(orders.NewMemoryStore()).get("/orders/"+orderID.String(), nil)

	assertStatus(t, response, http.StatusNotFound)
	body := decodeProblem(t, response)
	if body.Code != "ORDER_NOT_FOUND" {
		t.Errorf("code = %q, want ORDER_NOT_FOUND", body.Code)
	}
	if want := "Order " + orderID.String() + " does not exist."; body.Detail != want {
		t.Errorf("detail = %q, want %q", body.Detail, want)
	}
}

func TestAlreadyProcessedOrderMapsTo409(t *testing.T) {
	testApp := newTestApp(orders.NewMemoryStore())
	orderID := createOrder(t, testApp)
	testApp.post("/orders/" + orderID.String() + "/process")

	response := testApp.post("/orders/" + orderID.String() + "/process")

	assertStatus(t, response, http.StatusConflict)
	body := decodeProblem(t, response)
	if body.Code != "ORDER_ALREADY_PROCESSED" {
		t.Errorf("code = %q, want ORDER_ALREADY_PROCESSED", body.Code)
	}
	if want := "Order " + orderID.String() + " was already processed."; body.Detail != want {
		t.Errorf("detail = %q, want %q", body.Detail, want)
	}
}

type slowReadStore struct{ orders.Store }

func (s slowReadStore) Get(ctx context.Context, key string) ([]byte, bool, error) {
	record, found, err := s.Store.Get(ctx, key)
	time.Sleep(10 * time.Millisecond)
	return record, found, err
}

func TestConcurrentProcessingSucceedsOnce(t *testing.T) {
	testApp := newTestApp(slowReadStore{orders.NewMemoryStore()})
	orderID := createOrder(t, testApp)
	path := "/orders/" + orderID.String() + "/process"
	const callers = 16
	responses := make(chan *httptest.ResponseRecorder, callers)
	start := make(chan struct{})
	var wait sync.WaitGroup
	for range callers {
		wait.Go(func() {
			<-start
			responses <- testApp.post(path)
		})
	}
	close(start)
	wait.Wait()
	close(responses)

	var processed orders.Order
	successes := 0
	for response := range responses {
		if response.Code == http.StatusOK {
			successes++
			processed = decodeJSON[orders.Order](t, response)
			continue
		}
		assertStatus(t, response, http.StatusConflict)
		if body := decodeProblem(t, response); body.Code != "ORDER_ALREADY_PROCESSED" {
			t.Errorf("code = %q, want ORDER_ALREADY_PROCESSED", body.Code)
		}
	}
	if successes != 1 {
		t.Fatalf("got %d successful process responses, want 1", successes)
	}
	if processed.ProcessedAt == nil {
		t.Fatal("processedAt is null, want a timestamp")
	}
	stored := decodeJSON[orders.Order](t, testApp.get("/orders/"+orderID.String(), nil))
	if stored.ProcessedAt == nil || !stored.ProcessedAt.Equal(*processed.ProcessedAt) {
		t.Errorf("stored processedAt = %v, want %v", stored.ProcessedAt, processed.ProcessedAt)
	}
	if records := testApp.recordsAtOrAbove(t, slog.LevelWarn); len(records) != 0 {
		t.Errorf("got WARN+ records for concurrent expected outcomes: %v", records)
	}
}

func TestProcessingAPendingOrderSucceeds(t *testing.T) {
	testApp := newTestApp(orders.NewMemoryStore())
	orderID := createOrder(t, testApp)

	response := testApp.post("/orders/" + orderID.String() + "/process")

	assertStatus(t, response, http.StatusOK)
	order := decodeJSON[orders.Order](t, response)
	if order.Status != orders.StatusProcessed {
		t.Errorf("status = %q, want processed", order.Status)
	}
	if order.ProcessedAt == nil {
		t.Error("processedAt is null, want a timestamp")
	}
}

func TestExpectedOutcomesAreNotLoggedAsErrors(t *testing.T) {
	testApp := newTestApp(orders.NewMemoryStore())
	orderID := createOrder(t, testApp)
	testApp.post("/orders/" + orderID.String() + "/process")

	testApp.post("/orders/" + orderID.String() + "/process")
	testApp.get("/orders/"+uuid.New().String(), nil)

	if records := testApp.recordsAtOrAbove(t, slog.LevelWarn); len(records) != 0 {
		t.Fatalf("got WARN+ records for expected outcomes: %v", records)
	}
}
