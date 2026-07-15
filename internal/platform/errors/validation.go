package errors

import "fmt"

// FieldError describes a single field-level validation failure.
// It is serialised directly into the API error response.
type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// ValidationErrors is returned by the validation package when one or more
// struct fields fail their constraints.  It implements the error interface
// so it can be passed to Handle like any other application error.
type ValidationErrors struct {
	Fields []FieldError
}

func (e *ValidationErrors) Error() string {
	return fmt.Sprintf("validation failed on %d field(s)", len(e.Fields))
}
