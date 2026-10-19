package app_test

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"syscall"
	"testing"

	"github.com/rwaterman/error-handling-examples/go/app"
	"github.com/rwaterman/error-handling-examples/go/apperr"
	"github.com/rwaterman/error-handling-examples/go/orders"
)

const secret = "postgres://admin:hunter2@db-01:5432/orders"

func validOrder() map[string]any {
	return map[string]any{
		"customerEmail": "ada@example.com",
		"items":         []any{map[string]any{"sku": "BOOK-1", "quantity": 2}},
	}
}

func validOrderWith(key string, value any) map[string]any {
	order := validOrder()
	order[key] = value
	return order
}

func validOrderWithout(key string) map[string]any {
	order := validOrder()
	delete(order, key)
	return order
}

func items(entries ...map[string]any) []any {
	list := make([]any, 0, len(entries))
	for _, entry := range entries {
		list = append(list, entry)
	}
	return list
}

type failingStore struct{ err error }

func (s failingStore) Get(context.Context, string) ([]byte, bool, error) { return nil, false, s.err }

func (s failingStore) Put(context.Context, string, []byte) error { return s.err }

type panickingStore struct{ value any }

func (s panickingStore) Get(context.Context, string) ([]byte, bool, error) { panic(s.value) }

func (s panickingStore) Put(context.Context, string, []byte) error { panic(s.value) }

func refusedConnection(target string) error {
	dial := &net.OpError{Op: "dial", Net: "tcp", Err: os.NewSyscallError("connect", syscall.ECONNREFUSED)}
	return fmt.Errorf("connect to %s: %w", target, dial)
}

func readTimeout() error {
	return &net.OpError{Op: "read", Net: "tcp", Err: os.ErrDeadlineExceeded}
}

type testApp struct {
	handler http.Handler
	logs    *bytes.Buffer
}

func newTestApp(store orders.Store) *testApp {
	logs := &bytes.Buffer{}
	service := orders.NewService(orders.NewRepository(store))
	return &testApp{handler: app.New(service, slog.New(slog.NewJSONHandler(logs, nil))), logs: logs}
}

func (a *testApp) do(method, path string, body io.Reader, header http.Header) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, body)
	maps.Copy(req.Header, header)
	recorder := httptest.NewRecorder()
	a.handler.ServeHTTP(recorder, req)
	return recorder
}

func (a *testApp) get(path string, header http.Header) *httptest.ResponseRecorder {
	return a.do(http.MethodGet, path, nil, header)
}

func (a *testApp) post(path string) *httptest.ResponseRecorder {
	return a.do(http.MethodPost, path, nil, nil)
}

func (a *testApp) postJSON(t *testing.T, path string, payload any) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("encode payload: %v", err)
	}
	header := http.Header{"Content-Type": {"application/json"}}
	return a.do(http.MethodPost, path, bytes.NewReader(body), header)
}

// logRecords parses the JSON lines written by the app's logger.
func (a *testApp) logRecords(t *testing.T) []map[string]any {
	t.Helper()
	return parseLogLines(t, a.logs.String())
}

func (a *testApp) recordsAtOrAbove(t *testing.T, minimum slog.Level) []map[string]any {
	t.Helper()
	var matching []map[string]any
	for _, record := range a.logRecords(t) {
		var level slog.Level
		if err := level.UnmarshalText([]byte(fmt.Sprint(record["level"]))); err != nil {
			t.Fatalf("parse level of %v: %v", record, err)
		}
		if level >= minimum {
			matching = append(matching, record)
		}
	}
	return matching
}

func (a *testApp) singleErrorRecord(t *testing.T) map[string]any {
	t.Helper()
	records := a.recordsAtOrAbove(t, slog.LevelError)
	if len(records) != 1 {
		t.Fatalf("got %d ERROR records, want exactly 1: %v", len(records), records)
	}
	return records[0]
}

func parseLogLines(t *testing.T, output string) []map[string]any {
	t.Helper()
	var records []map[string]any
	for line := range strings.Lines(output) {
		var record map[string]any
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			t.Fatalf("parse log line %q: %v", line, err)
		}
		records = append(records, record)
	}
	return records
}

type problemBody struct {
	Type      string              `json:"type"`
	Title     string              `json:"title"`
	Status    int                 `json:"status"`
	Detail    string              `json:"detail"`
	Code      string              `json:"code"`
	Instance  string              `json:"instance"`
	RequestID string              `json:"requestId"`
	Errors    []apperr.FieldError `json:"errors"`
}

func decodeProblem(t *testing.T, response *httptest.ResponseRecorder) problemBody {
	t.Helper()
	if got := response.Header().Get("Content-Type"); got != "application/problem+json" {
		t.Fatalf("Content-Type = %q, want application/problem+json", got)
	}
	var body problemBody
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode problem %q: %v", response.Body.String(), err)
	}
	return body
}

func decodeJSON[T any](t *testing.T, response *httptest.ResponseRecorder) T {
	t.Helper()
	if got := response.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", got)
	}
	var body T
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body %q: %v", response.Body.String(), err)
	}
	return body
}

func fieldsOf(body problemBody) []string {
	fields := make([]string, 0, len(body.Errors))
	for _, fieldError := range body.Errors {
		fields = append(fields, fieldError.Field)
	}
	return fields
}

func assertStatus(t *testing.T, response *httptest.ResponseRecorder, want int) {
	t.Helper()
	if response.Code != want {
		t.Fatalf("status = %d, want %d; body: %s", response.Code, want, response.Body.String())
	}
}
