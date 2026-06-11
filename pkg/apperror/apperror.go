package apperror

import (
	"errors"
	"net/http"

	"gorm.io/gorm"
)

type Code string

const (
	CodeNotFound     Code = "NOT_FOUND"
	CodeInvalidInput Code = "INVALID_INPUT"
	CodeConflict     Code = "CONFLICT"
	CodeUnauthorized Code = "UNAUTHORIZED"
	CodeForbidden    Code = "FORBIDDEN"
	CodeInternal     Code = "INTERNAL"
)

type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

type AppError struct {
	Code    Code         `json:"code"`
	Message string       `json:"message"`
	Fields  []FieldError `json:"fields,omitempty"`
	Err     error        `json:"-"`
}

func (e *AppError) Error() string {
	if e.Err != nil {
		return e.Message + ": " + e.Err.Error()
	}
	return e.Message
}

func (e *AppError) Unwrap() error { return e.Err }

func (e *AppError) HTTPStatus() int {
	switch e.Code {
	case CodeNotFound:
		return http.StatusNotFound
	case CodeInvalidInput:
		return http.StatusBadRequest
	case CodeConflict:
		return http.StatusConflict
	case CodeUnauthorized:
		return http.StatusUnauthorized
	case CodeForbidden:
		return http.StatusForbidden
	default:
		return http.StatusInternalServerError
	}
}

func NotFound(msg string) *AppError     { return &AppError{Code: CodeNotFound, Message: msg} }
func InvalidInput(msg string) *AppError { return &AppError{Code: CodeInvalidInput, Message: msg} }
func Conflict(msg string) *AppError     { return &AppError{Code: CodeConflict, Message: msg} }
func Unauthorized(msg string) *AppError { return &AppError{Code: CodeUnauthorized, Message: msg} }
func Forbidden(msg string) *AppError    { return &AppError{Code: CodeForbidden, Message: msg} }

func Internal(err error) *AppError {
	return &AppError{Code: CodeInternal, Message: "internal server error", Err: err}
}

func Validation(msg string, fields []FieldError) *AppError {
	return &AppError{Code: CodeInvalidInput, Message: msg, Fields: fields}
}

// From normalizes any error into an *AppError. Already-typed AppErrors pass
// through; known sentinels (gorm.ErrRecordNotFound) are mapped; everything
// else becomes Internal so the cause is preserved for logging but hidden from
// the client.
func From(err error) *AppError {
	if err == nil {
		return nil
	}
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return NotFound("resource not found")
	}
	return Internal(err)
}
