package flag

import (
	"strings"
)

// MultiError aggregates multiple validation errors deterministically.
type MultiError struct{ errs []error }

// Error implements error.
func (m *MultiError) Error() string {
	if m == nil || len(m.errs) == 0 {
		return ""
	}
	parts := make([]string, 0, len(m.errs))
	for _, e := range m.errs {
		parts = append(parts, e.Error())
	}
	return strings.Join(parts, "; ")
}

// Errors returns a copy of the underlying errors slice.
func (m *MultiError) Errors() []error { return append([]error(nil), m.errs...) }

// Append adds a non-nil error.
func (m *MultiError) Append(err error) {
	if err != nil {
		m.errs = append(m.errs, err)
	}
}

// HasErrors returns true if at least one error recorded.
func (m *MultiError) HasErrors() bool { return m != nil && len(m.errs) > 0 }

// Unwrap returns the underlying errors slice to support errors.Is / errors.As
// style matching introduced in Go 1.20+ which walks multiple errors when
// Unwrap() []error is provided.
func (m *MultiError) Unwrap() []error {
	if m == nil {
		return nil
	}
	return m.errs
}
