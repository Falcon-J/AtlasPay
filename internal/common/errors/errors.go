package errors

import (
	"encoding/json"
	"net/http"
)

// AppError represents an application error with HTTP status code
type AppError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}

func (e *AppError) Error() string {
	return e.Message
}

// Common errors
var (
	ErrBadRequest         = &AppError{Code: http.StatusBadRequest, Message: "Bad request"}
	ErrUnauthorized       = &AppError{Code: http.StatusUnauthorized, Message: "Unauthorized"}
	ErrForbidden          = &AppError{Code: http.StatusForbidden, Message: "Forbidden"}
	ErrNotFound           = &AppError{Code: http.StatusNotFound, Message: "Resource not found"}
	ErrConflict           = &AppError{Code: http.StatusConflict, Message: "Resource conflict"}
	ErrInternalServer     = &AppError{Code: http.StatusInternalServerError, Message: "Internal server error"}
	ErrServiceUnavailable = &AppError{Code: http.StatusServiceUnavailable, Message: "Service unavailable"}
	ErrInvalidCredentials = &AppError{Code: http.StatusUnauthorized, Message: "Invalid credentials"}
	ErrTokenExpired       = &AppError{Code: http.StatusUnauthorized, Message: "Token expired"}
	ErrInvalidToken       = &AppError{Code: http.StatusUnauthorized, Message: "Invalid token"}
	ErrInsufficientStock  = &AppError{Code: http.StatusBadRequest, Message: "Insufficient stock"}
	ErrPaymentFailed      = &AppError{Code: http.StatusPaymentRequired, Message: "Payment failed"}
	ErrOrderNotFound      = &AppError{Code: http.StatusNotFound, Message: "Order not found"}
	ErrUserExists         = &AppError{Code: http.StatusConflict, Message: "User already exists"}
)

// New creates a new AppError
func New(code int, message string) *AppError {
	return &AppError{Code: code, Message: message}
}

// WithDetails adds details to an error
func (e *AppError) WithDetails(details string) *AppError {
	return &AppError{
		Code:    e.Code,
		Message: e.Message,
		Details: details,
	}
}

// WriteJSON writes the error as JSON response
func (e *AppError) WriteJSON(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(e.Code)
	json.NewEncoder(w).Encode(e)
}

// ErrorResponse is a helper to write error response
type ErrorResponse struct {
	Success bool      `json:"success"`
	Error   *AppError `json:"error"`
}

// WriteError writes an error response
func WriteError(w http.ResponseWriter, err *AppError) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(err.Code)
	json.NewEncoder(w).Encode(ErrorResponse{Success: false, Error: err})
}
