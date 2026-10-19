package app_test

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"uuid"

	"github.com/rwaterman/error-handling-examples/go/orders"
)

func TestDependencyOutageReturns503WithoutLeakingInternals(t *testing.T) {
	testApp := newTestApp(failingStore{err: refusedConnection(secret)})
	path := "/orders/" + uuid.New().String()

	response := testApp.get(path, http.Header{"X-Request-Id": {"req-503"}})

	assertStatus(t, response, http.StatusServiceUnavailable)
	body := decodeProblem(t, response)
	if body.Code != "DEPENDENCY_UNAVAILABLE" {
		t.Errorf("code = %q, want DEPENDENCY_UNAVAILABLE", body.Code)
	}
	if body.Detail != "A required dependency is unavailable: order-store." {
		t.Errorf("detail = %q", body.Detail)
	}
	if body.RequestID != "req-503" {
		t.Errorf("requestId = %q, want req-503", body.RequestID)
	}
	if strings.Contains(response.Body.String(), secret) {
		t.Errorf("response body leaks the secret: %s", response.Body.String())
	}
	record := testApp.singleErrorRecord(t)
	assertRecord(t, record, "req-503", "DEPENDENCY_UNAVAILABLE", path)
	if logged := fmt.Sprint(record["error"]); !strings.Contains(logged, "connection refused") {
		t.Errorf("logged error = %q, want the driver error attached", logged)
	}
}

func TestUnexpectedErrorReturnsGeneric500AndLogsTheRealOne(t *testing.T) {
	testApp := newTestApp(failingStore{err: errors.New(secret)})
	path := "/orders/" + uuid.New().String()

	response := testApp.get(path, http.Header{"X-Request-Id": {"req-500"}})

	assertGeneric500(t, response, "req-500")
	record := testApp.singleErrorRecord(t)
	assertRecord(t, record, "req-500", "INTERNAL_ERROR", path)
	if logged := fmt.Sprint(record["error"]); !strings.Contains(logged, secret) {
		t.Errorf("logged error = %q, want it to contain the real error", logged)
	}
}

func TestPanicInAHandlerReturnsTheGeneric500AndIsLogged(t *testing.T) {
	testApp := newTestApp(panickingStore{value: secret})
	path := "/orders/" + uuid.New().String()

	response := testApp.get(path, http.Header{"X-Request-Id": {"req-panic"}})

	assertGeneric500(t, response, "req-panic")
	if got := response.Header().Get("X-Request-Id"); got != "req-panic" {
		t.Errorf("x-request-id header = %q, want req-panic", got)
	}
	record := testApp.singleErrorRecord(t)
	assertRecord(t, record, "req-panic", "INTERNAL_ERROR", path)
	if logged := fmt.Sprint(record["error"]); !strings.Contains(logged, secret) {
		t.Errorf("logged error = %q, want it to contain the panic value", logged)
	}
	if stack := fmt.Sprint(record["stack"]); !strings.Contains(stack, "panickingStore.Get") {
		t.Errorf("logged stack does not show where the panic happened: %q", stack)
	}
}

func TestRequestIDIsGeneratedWhenTheCallerSendsNone(t *testing.T) {
	testApp := newTestApp(failingStore{err: errors.New("boom")})

	response := testApp.get("/orders/"+uuid.New().String(), nil)

	requestID := decodeProblem(t, response).RequestID
	if _, err := uuid.Parse(requestID); err != nil {
		t.Errorf("requestId = %q, want a generated UUID", requestID)
	}
	if got := response.Header().Get("X-Request-Id"); got != requestID {
		t.Errorf("x-request-id header = %q, want %q", got, requestID)
	}
}

func TestUntrustedRequestIDIsReplaced(t *testing.T) {
	testApp := newTestApp(failingStore{err: errors.New("boom")})

	response := testApp.get("/orders/"+uuid.New().String(), http.Header{"X-Request-Id": {"has space"}})

	requestID := decodeProblem(t, response).RequestID
	if requestID == "has space" || requestID == "" {
		t.Errorf("requestId = %q, want a generated ID", requestID)
	}
	if got := response.Header().Get("X-Request-Id"); got != requestID {
		t.Errorf("x-request-id header = %q, want %q", got, requestID)
	}
}

func TestUnknownRouteUsesTheSameProblemFormat(t *testing.T) {
	response := newTestApp(orders.NewMemoryStore()).get("/nope", nil)

	assertStatus(t, response, http.StatusNotFound)
	body := decodeProblem(t, response)
	if body.Code != "ROUTE_NOT_FOUND" {
		t.Errorf("code = %q, want ROUTE_NOT_FOUND", body.Code)
	}
	if body.Detail != "Not Found" || body.Title != "Not Found" || body.Instance != "/nope" {
		t.Errorf("problem = %+v, want detail and title Not Found and instance /nope", body)
	}
}

func assertGeneric500(t *testing.T, response *httptest.ResponseRecorder, requestID string) {
	t.Helper()
	assertStatus(t, response, http.StatusInternalServerError)
	body := decodeProblem(t, response)
	if body.Code != "INTERNAL_ERROR" {
		t.Errorf("code = %q, want INTERNAL_ERROR", body.Code)
	}
	if body.Detail != "An unexpected error occurred." {
		t.Errorf("detail = %q, want the generic detail", body.Detail)
	}
	if body.RequestID != requestID {
		t.Errorf("requestId = %q, want %q", body.RequestID, requestID)
	}
	if strings.Contains(response.Body.String(), secret) {
		t.Errorf("response body leaks the secret: %s", response.Body.String())
	}
}

func assertRecord(t *testing.T, record map[string]any, requestID, code, path string) {
	t.Helper()
	want := map[string]string{"request_id": requestID, "code": code, "path": path}
	for key, value := range want {
		if record[key] != value {
			t.Errorf("log record %s = %v, want %q", key, record[key], value)
		}
	}
	if record["error"] == nil || record["error"] == "" {
		t.Errorf("log record has no error attached: %v", record)
	}
}
