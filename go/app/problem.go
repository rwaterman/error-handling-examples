package app

import (
	"encoding/json/v2"
	"fmt"
	"net/http"
	"strings"

	"github.com/rwaterman/error-handling-examples/go/apperr"
)

const (
	jsonContentType    = "application/json"
	problemContentType = "application/problem+json"
)

// problem is an RFC 9457 problem details body with the contract's extensions.
type problem struct {
	Type      string              `json:"type"`
	Title     string              `json:"title"`
	Status    int                 `json:"status"`
	Detail    string              `json:"detail"`
	Code      string              `json:"code"`
	Instance  string              `json:"instance"`
	RequestID string              `json:"requestId"`
	Errors    []apperr.FieldError `json:"errors,omitempty"`
}

func writeProblem(w http.ResponseWriter, r *http.Request, status int, code, detail string, fields []apperr.FieldError) error {
	return write(w, status, problemContentType, problem{
		Type:      "about:blank",
		Title:     http.StatusText(status),
		Status:    status,
		Detail:    detail,
		Code:      code,
		Instance:  strings.ToValidUTF8(r.URL.Path, "�"),
		RequestID: requestIDFrom(r.Context()),
		Errors:    fields,
	})
}

func writeJSON(w http.ResponseWriter, status int, value any) error {
	return write(w, status, jsonContentType, value)
}

// write encodes value before touching w, so an encoding failure leaves the
// response unwritten for the caller to handle.
func write(w http.ResponseWriter, status int, contentType string, value any) error {
	body, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("encode %T response: %w", value, err)
	}
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(status)
	// A failed write means the client is gone and the status is already sent.
	w.Write(body)
	return nil
}
