package service

import (
	"net/http"
	"time"
)

// PublicError is an error type safe to expose to dashboard users.
// It carries an HTTP status code and an optional retry delay.
type PublicError struct {
	Status     int
	Message    string
	RetryAfter time.Duration
}

func (e *PublicError) Error() string {
	return e.Message
}

func (e *PublicError) PublicMessage() string {
	if e == nil {
		return ""
	}
	return e.Message
}
func (e *PublicError) RetryDelay() time.Duration {
	if e == nil {
		return 0
	}
	return e.RetryAfter
}

func (e *PublicError) StatusCode() int {
	if e == nil || e.Status == 0 {
		return http.StatusBadRequest
	}
	return e.Status
}
