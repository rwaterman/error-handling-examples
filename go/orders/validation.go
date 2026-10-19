package orders

import (
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"net/mail"
	"slices"
	"strings"
	"unicode/utf8"
	"uuid"

	"github.com/rwaterman/error-handling-examples/go/apperr"
)

const (
	maxItems     = 50
	maxSKULength = 64
	maxQuantity  = 100
)

// CreateOrderRequest is a create-order body that has passed validation.
// DecodeCreateOrderRequest is the only code that builds one from client input.
type CreateOrderRequest struct {
	CustomerEmail string
	Items         []Item
}

type createOrderInput struct {
	CustomerEmail *string     `json:"customerEmail"`
	Items         []itemInput `json:"items"`
}

type itemInput struct {
	SKU      *string `json:"sku"`
	Quantity *int    `json:"quantity"`
}

type fieldErrors []apperr.FieldError

func (f *fieldErrors) add(field, message string) {
	*f = append(*f, apperr.FieldError{Field: field, Message: message})
}

// DecodeCreateOrderRequest parses and validates a create-order body. Every
// failure is a *apperr.ValidationError that lists each invalid field.
func DecodeCreateOrderRequest(body io.Reader) (CreateOrderRequest, error) {
	var input createOrderInput
	if err := json.UnmarshalRead(body, &input, json.RejectUnknownMembers(true)); err != nil {
		return CreateOrderRequest{}, &apperr.ValidationError{Fields: []apperr.FieldError{decodeFailure(err)}}
	}
	return input.validate()
}

// ParseOrderID validates the orderId path segment.
func ParseOrderID(raw string) (uuid.UUID, error) {
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.UUID{}, &apperr.ValidationError{
			Fields: []apperr.FieldError{{Field: "orderId", Message: "must be a UUID"}},
		}
	}
	return id, nil
}

func decodeFailure(err error) apperr.FieldError {
	semantic, ok := errors.AsType[*json.SemanticError](err)
	if !ok || semantic.JSONPointer == "" {
		return apperr.FieldError{Field: "body", Message: "must be a single valid JSON object"}
	}
	field := strings.Join(slices.Collect(semantic.JSONPointer.Tokens()), ".")
	if errors.Is(err, json.ErrUnknownName) {
		return apperr.FieldError{Field: field, Message: "is not an allowed field"}
	}
	return apperr.FieldError{Field: field, Message: "has the wrong type"}
}

func (in createOrderInput) validate() (CreateOrderRequest, error) {
	var errs fieldErrors
	switch {
	case in.CustomerEmail == nil:
		errs.add("customerEmail", "is required")
	case !isEmailAddress(*in.CustomerEmail):
		errs.add("customerEmail", "must be a valid email address")
	}
	switch {
	case in.Items == nil:
		errs.add("items", "is required")
	case len(in.Items) == 0 || len(in.Items) > maxItems:
		errs.add("items", fmt.Sprintf("must contain between 1 and %d entries", maxItems))
	}
	items := make([]Item, 0, len(in.Items))
	for index, input := range in.Items {
		items = append(items, input.validate(fmt.Sprintf("items.%d", index), &errs))
	}
	if len(errs) > 0 {
		return CreateOrderRequest{}, &apperr.ValidationError{Fields: errs}
	}
	return CreateOrderRequest{CustomerEmail: *in.CustomerEmail, Items: items}, nil
}

func (in itemInput) validate(path string, errs *fieldErrors) Item {
	var item Item
	switch {
	case in.SKU == nil:
		errs.add(path+".sku", "is required")
	case utf8.RuneCountInString(*in.SKU) < 1 || utf8.RuneCountInString(*in.SKU) > maxSKULength:
		errs.add(path+".sku", fmt.Sprintf("must be between 1 and %d characters", maxSKULength))
	default:
		item.SKU = *in.SKU
	}
	switch {
	case in.Quantity == nil:
		errs.add(path+".quantity", "is required")
	case *in.Quantity < 1 || *in.Quantity > maxQuantity:
		errs.add(path+".quantity", fmt.Sprintf("must be between 1 and %d", maxQuantity))
	default:
		item.Quantity = *in.Quantity
	}
	return item
}

func isEmailAddress(candidate string) bool {
	address, err := mail.ParseAddress(candidate)
	return err == nil && address.Address == candidate
}
