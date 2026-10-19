package app_test

import (
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/rwaterman/error-handling-examples/go/orders"
)

func TestInvalidOrderIsRejectedWith400NamingTheField(t *testing.T) {
	tests := []struct {
		name    string
		payload map[string]any
		field   string
	}{
		{"missing customerEmail", validOrderWithout("customerEmail"), "customerEmail"},
		{"malformed customerEmail", validOrderWith("customerEmail", "not-an-email"), "customerEmail"},
		{"empty items", validOrderWith("items", []any{}), "items"},
		{"zero quantity", validOrderWith("items", items(map[string]any{"sku": "BOOK-1", "quantity": 0})), "items.0.quantity"},
		{"empty sku", validOrderWith("items", items(map[string]any{"sku": "", "quantity": 1})), "items.0.sku"},
		{"quantity of the wrong type", validOrderWith("items", items(map[string]any{"sku": "BOOK-1", "quantity": "2"})), "items.0.quantity"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := newTestApp(orders.NewMemoryStore()).postJSON(t, "/orders", test.payload)

			assertStatus(t, response, http.StatusBadRequest)
			body := decodeProblem(t, response)
			if body.Code != "VALIDATION_FAILED" {
				t.Errorf("code = %q, want VALIDATION_FAILED", body.Code)
			}
			if !slices.Contains(fieldsOf(body), test.field) {
				t.Errorf("errors name %v, want them to include %q", fieldsOf(body), test.field)
			}
		})
	}
}

func TestInvalidOrderNeverReachesTheStore(t *testing.T) {
	store := orders.NewMemoryStore()

	newTestApp(store).postJSON(t, "/orders", validOrderWith("customerEmail", "nope"))

	if store.Len() != 0 {
		t.Fatalf("store holds %d records, want 0", store.Len())
	}
}

func TestUnknownFieldIsRejected(t *testing.T) {
	tests := []struct {
		name    string
		payload map[string]any
		field   string
	}{
		{"at the top level", validOrderWith("discountCode", "FREE"), "discountCode"},
		{"inside an item", validOrderWith("items", items(map[string]any{"sku": "BOOK-1", "quantity": 1, "color": "red"})), "items.0.color"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := newTestApp(orders.NewMemoryStore()).postJSON(t, "/orders", test.payload)

			assertStatus(t, response, http.StatusBadRequest)
			body := decodeProblem(t, response)
			if body.Code != "VALIDATION_FAILED" {
				t.Errorf("code = %q, want VALIDATION_FAILED", body.Code)
			}
			if got := fieldsOf(body); !slices.Equal(got, []string{test.field}) {
				t.Errorf("errors name %v, want [%s]", got, test.field)
			}
		})
	}
}

func TestMalformedJSONIsAValidationFailure(t *testing.T) {
	header := http.Header{"Content-Type": {"application/json"}}

	response := newTestApp(orders.NewMemoryStore()).do(http.MethodPost, "/orders", strings.NewReader("{not json"), header)

	assertStatus(t, response, http.StatusBadRequest)
	body := decodeProblem(t, response)
	if body.Code != "VALIDATION_FAILED" {
		t.Errorf("code = %q, want VALIDATION_FAILED", body.Code)
	}
	if got := fieldsOf(body); !slices.Equal(got, []string{"body"}) {
		t.Errorf("errors name %v, want [body]", got)
	}
}

func TestMalformedOrderIDIsRejectedBeforeLookup(t *testing.T) {
	response := newTestApp(failingStore{err: refusedConnection(secret)}).get("/orders/not-a-uuid", nil)

	assertStatus(t, response, http.StatusBadRequest)
	if got := fieldsOf(decodeProblem(t, response)); !slices.Equal(got, []string{"orderId"}) {
		t.Errorf("errors name %v, want [orderId]", got)
	}
}

func TestValidOrderIsCreated(t *testing.T) {
	response := newTestApp(orders.NewMemoryStore()).postJSON(t, "/orders", validOrder())

	assertStatus(t, response, http.StatusCreated)
	body := decodeJSON[map[string]any](t, response)
	if body["customerEmail"] != "ada@example.com" {
		t.Errorf("customerEmail = %v, want ada@example.com", body["customerEmail"])
	}
	if body["status"] != "pending" {
		t.Errorf("status = %v, want pending", body["status"])
	}
	if processedAt, present := body["processedAt"]; !present || processedAt != nil {
		t.Errorf("processedAt = %v (present %t), want null", processedAt, present)
	}
	if createdAt, _ := body["createdAt"].(string); !strings.HasSuffix(createdAt, "Z") {
		t.Errorf("createdAt = %q, want a UTC timestamp ending in Z", createdAt)
	}
}
