package api

import (
	"context"
	"errors"
	"net/http"

	"github.com/anthropics/claude-workflow/runtime/contracts"
)

// API-specific errors.
var (
	// ErrRunExists is returned when trying to create a run with an existing ID.
	ErrRunExists = errors.New("run already exists")

	// ErrNotImplemented is returned for endpoints not yet implemented.
	ErrNotImplemented = errors.New("not implemented in V1")
)

// ErrorCode represents an API error code.
type ErrorCode string

// Error codes for API responses.
const (
	CodeInvalidInput   ErrorCode = "invalid_input"
	CodeDAGCycle       ErrorCode = "dag_cycle"
	CodeDAGInvalid     ErrorCode = "dag_invalid"
	CodeDepNotFound    ErrorCode = "dep_not_found"
	CodeRunNotFound    ErrorCode = "run_not_found"
	CodeRunExists      ErrorCode = "run_exists"
	CodeRunCompleted   ErrorCode = "run_completed"
	CodeRunAborted     ErrorCode = "run_aborted"
	CodeBudgetExceeded ErrorCode = "budget_exceeded"
	CodeTaskFailed     ErrorCode = "task_failed"
	CodeDeadlock       ErrorCode = "deadlock"
	CodeCancelled      ErrorCode = "cancelled"
	CodeTimeout        ErrorCode = "timeout"
	CodeNotImplemented ErrorCode = "not_implemented"
	CodeInternalError  ErrorCode = "internal_error"
)

// HTTPError represents an error with an associated HTTP status code.
type HTTPError struct {
	StatusCode int
	Code       ErrorCode
	Err        error
}

func (e *HTTPError) Error() string {
	return e.Err.Error()
}

func (e *HTTPError) Unwrap() error {
	return e.Err
}

// MapError maps a domain error to an HTTPError.
func MapError(err error) *HTTPError {
	if err == nil {
		return nil
	}

	// Check for specific error types
	switch {
	case errors.Is(err, contracts.ErrInvalidInput):
		return &HTTPError{http.StatusBadRequest, CodeInvalidInput, err}

	case errors.Is(err, contracts.ErrDAGCycle):
		return &HTTPError{http.StatusUnprocessableEntity, CodeDAGCycle, err}

	case errors.Is(err, contracts.ErrDAGInvalid):
		return &HTTPError{http.StatusUnprocessableEntity, CodeDAGInvalid, err}

	case errors.Is(err, contracts.ErrDepNotFound):
		return &HTTPError{http.StatusUnprocessableEntity, CodeDepNotFound, err}

	case errors.Is(err, contracts.ErrRunNotFound):
		return &HTTPError{http.StatusNotFound, CodeRunNotFound, err}

	case errors.Is(err, ErrRunExists):
		return &HTTPError{http.StatusConflict, CodeRunExists, err}

	case errors.Is(err, contracts.ErrRunCompleted):
		return &HTTPError{http.StatusConflict, CodeRunCompleted, err}

	case errors.Is(err, contracts.ErrRunAborted):
		return &HTTPError{http.StatusConflict, CodeRunAborted, err}

	case errors.Is(err, contracts.ErrBudgetExceeded):
		return &HTTPError{http.StatusUnprocessableEntity, CodeBudgetExceeded, err}

	case errors.Is(err, contracts.ErrTaskFailed):
		return &HTTPError{http.StatusInternalServerError, CodeTaskFailed, err}

	case errors.Is(err, contracts.ErrDeadlock):
		return &HTTPError{http.StatusInternalServerError, CodeDeadlock, err}

	case errors.Is(err, context.Canceled),
		errors.Is(err, contracts.ErrTaskCancelled):
		// 499: nginx convention for "client closed request"
		return &HTTPError{499, CodeCancelled, err}

	case errors.Is(err, context.DeadlineExceeded),
		errors.Is(err, contracts.ErrTaskTimeout):
		return &HTTPError{http.StatusGatewayTimeout, CodeTimeout, err}

	case errors.Is(err, ErrNotImplemented):
		return &HTTPError{http.StatusNotImplemented, CodeNotImplemented, err}

	default:
		return &HTTPError{http.StatusInternalServerError, CodeInternalError, err}
	}
}

// WriteError writes an error response to the HTTP response writer.
func WriteError(w http.ResponseWriter, err error) {
	httpErr := MapError(err)
	if httpErr == nil {
		return
	}

	resp := ErrorDTO{
		Code:    string(httpErr.Code),
		Message: httpErr.Error(),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpErr.StatusCode)
	writeJSON(w, resp)
}
