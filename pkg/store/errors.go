package store

import (
	"errors"
	"fmt"
	"strconv"
)

// Sentinel errors callers can match with errors.Is.
var (
	// ErrSessionExpired means the borrowed token is no longer valid; re-import.
	ErrSessionExpired = errors.New("session expired or invalid — re-capture from Configurator and `token import` again")
	// ErrNoLicense means the account holds no license for the title.
	ErrNoLicense = errors.New("account holds no license for this title")
	// ErrNotServed means the requested build is no longer served by Apple.
	ErrNotServed = errors.New("requested build is no longer served")
	// ErrUnavailable means the store reported a temporary failure.
	ErrUnavailable = errors.New("title temporarily unavailable")
	// ErrEmptyResult means a success response carried no download ticket.
	ErrEmptyResult = errors.New("store returned no download item")
)

// FailureError wraps a non-empty failureType from the store.
type FailureError struct {
	Type    string
	Message string
}

func (e *FailureError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("store failureType %s: %s", e.Type, e.Message)
	}
	return "store failureType " + e.Type
}

// Is lets callers match a FailureError against the sentinel for its category.
func (e *FailureError) Is(target error) bool {
	return target == classifyFailure(e.Type)
}

// classifyFailure maps a failureType code to a sentinel category.
func classifyFailure(code string) error {
	switch code {
	case "2042", "2034", "2059_signin", "2060":
		return ErrSessionExpired
	case "9610":
		return ErrNoLicense
	case "2059":
		return ErrUnavailable
	default:
		return nil
	}
}

// itoa renders an int64 in base 10.
func itoa(v int64) string { return strconv.FormatInt(v, 10) }
