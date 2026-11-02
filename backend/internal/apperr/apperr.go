// Package apperr defines typed business errors; handlers map them to RFC 9457
// Problem Details through a single mapper.
package apperr

import (
	"errors"
	"fmt"
	"time"
)

type Kind int

const (
	KindInternal Kind = iota
	KindBadRequest
	KindValidation
	KindUnauthorized
	KindForbidden
	KindNotFound
	KindConflict
	KindRateLimited
	KindUnavailable
	KindBadGateway
)

type FieldError struct {
	Field string `json:"field"`
	Code  string `json:"code"`
}

type Error struct {
	Kind       Kind
	Code       string
	Detail     string
	Fields     []FieldError
	RetryAfter time.Duration
	Err        error
}

func (e *Error) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Detail, e.Err)
	}
	return e.Code + ": " + e.Detail
}

func (e *Error) Unwrap() error { return e.Err }

func As(err error) (*Error, bool) {
	var e *Error
	ok := errors.As(err, &e)
	return e, ok
}

func IsKind(err error, k Kind) bool {
	e, ok := As(err)
	return ok && e.Kind == k
}

func New(k Kind, code, detail string) *Error { return &Error{Kind: k, Code: code, Detail: detail} }

func BadRequest(code, detail string) *Error { return New(KindBadRequest, code, detail) }
func Unauthorized(code string) *Error       { return New(KindUnauthorized, code, "") }
func Forbidden(code, detail string) *Error  { return New(KindForbidden, code, detail) }
func NotFound(what string) *Error           { return New(KindNotFound, "not_found", what+" not found") }
func Conflict(code, detail string) *Error   { return New(KindConflict, code, detail) }
func Unavailable(code string, err error) *Error {
	return &Error{Kind: KindUnavailable, Code: code, Detail: "service temporarily unavailable", Err: err}
}

func RateLimited(retry time.Duration) *Error {
	return &Error{Kind: KindRateLimited, Code: "rate_limited", Detail: "too many requests", RetryAfter: retry}
}

// Validation builds a 422 error with field details.
func Validation(fields ...FieldError) *Error {
	return &Error{Kind: KindValidation, Code: "validation_failed", Detail: "request validation failed", Fields: fields}
}

// Invalid is a shorthand for a single field validation error.
func Invalid(field, code string) *Error { return Validation(FieldError{Field: field, Code: code}) }

// Unprocessable is a 422 business rule violation without a specific field.
func Unprocessable(code, detail string) *Error { return New(KindValidation, code, detail) }

func Internal(err error) *Error {
	return &Error{Kind: KindInternal, Code: "internal_error", Detail: "internal error", Err: err}
}

// Collector accumulates field errors.
type Collector struct{ fields []FieldError }

func (c *Collector) Add(field, code string) { c.fields = append(c.fields, FieldError{field, code}) }
func (c *Collector) Err() error {
	if len(c.fields) == 0 {
		return nil
	}
	return Validation(c.fields...)
}
