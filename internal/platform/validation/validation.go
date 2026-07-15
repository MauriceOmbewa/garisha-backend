// Package validation provides struct validation and JSON request body decoding.
//
// It wraps go-playground/validator and returns typed errors that integrate
// directly with the errors and response packages.
//
// Usage — decode and validate in one call:
//
//	var req CreateVehicleRequest
//	if err := validation.DecodeJSON(r, &req); err != nil {
//	    apperr.Handle(w, r, err, log)
//	    return
//	}
package validation

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"sync"

	"github.com/go-playground/validator/v10"

	apperr "github.com/MauriceOmbewa/garisha-backend/internal/platform/errors"
)

// ─── Singleton validator ──────────────────────────────────────────────────────

var (
	once     sync.Once
	validate *validator.Validate
)

// instance returns the package-level validator, initialising it once.
// Using a singleton avoids rebuilding the reflection cache on every request.
func instance() *validator.Validate {
	once.Do(func() {
		validate = validator.New()

		// Register a custom tag name function so error messages use the JSON
		// field name (e.g. "first_name") instead of the Go struct field name
		// (e.g. "FirstName").
		validate.RegisterTagNameFunc(func(fld reflect.StructField) string {
			name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
			if name == "-" {
				return ""
			}
			return name
		})
	})

	return validate
}

// ─── Decode & Validate ────────────────────────────────────────────────────────

// DecodeJSON decodes the JSON request body into dst and then validates the
// resulting struct.  It returns:
//
//   - *apperr.AppError  (KindBadRequest)  on malformed JSON
//   - *apperr.ValidationErrors            on constraint violations
//   - nil                                 on success
func DecodeJSON(r *http.Request, dst any) error {
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		return apperr.BadRequest(fmt.Sprintf("invalid request body: %s", sanitiseJSONError(err)))
	}

	return Struct(dst)
}

// Struct validates a pre-populated struct.  Returns nil on success or a
// *apperr.ValidationErrors containing one FieldError per failing constraint.
func Struct(s any) error {
	if err := instance().Struct(s); err != nil {
		var ve validator.ValidationErrors
		if errors.As(err, &ve) {
			return buildValidationErrors(ve)
		}
		// Structural errors (e.g. non-struct passed) — treat as internal.
		return apperr.Internal("validation: unexpected error", err)
	}

	return nil
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

// buildValidationErrors converts go-playground errors into our FieldError list.
func buildValidationErrors(ve validator.ValidationErrors) *apperr.ValidationErrors {
	fields := make([]apperr.FieldError, 0, len(ve))

	for _, e := range ve {
		fields = append(fields, apperr.FieldError{
			Field:   e.Field(),
			Message: fieldMessage(e),
		})
	}

	return &apperr.ValidationErrors{Fields: fields}
}

// fieldMessage produces a readable message for a single validation failure.
func fieldMessage(e validator.FieldError) string {
	switch e.Tag() {
	case "required":
		return "this field is required"
	case "email":
		return "must be a valid email address"
	case "min":
		return fmt.Sprintf("must be at least %s characters", e.Param())
	case "max":
		return fmt.Sprintf("must be at most %s characters", e.Param())
	case "len":
		return fmt.Sprintf("must be exactly %s characters", e.Param())
	case "url":
		return "must be a valid URL"
	case "uuid4":
		return "must be a valid UUID"
	case "oneof":
		return fmt.Sprintf("must be one of: %s", strings.ReplaceAll(e.Param(), " ", ", "))
	case "numeric":
		return "must contain only numbers"
	case "alphanum":
		return "must contain only letters and numbers"
	case "gt":
		return fmt.Sprintf("must be greater than %s", e.Param())
	case "gte":
		return fmt.Sprintf("must be greater than or equal to %s", e.Param())
	case "lt":
		return fmt.Sprintf("must be less than %s", e.Param())
	case "lte":
		return fmt.Sprintf("must be less than or equal to %s", e.Param())
	default:
		return fmt.Sprintf("failed validation: %s", e.Tag())
	}
}

// sanitiseJSONError extracts a human-readable description from a JSON decode
// error without leaking raw Go internals into the client response.
func sanitiseJSONError(err error) string {
	var syntaxErr *json.SyntaxError
	if errors.As(err, &syntaxErr) {
		return fmt.Sprintf("syntax error at position %d", syntaxErr.Offset)
	}

	var unmarshalErr *json.UnmarshalTypeError
	if errors.As(err, &unmarshalErr) {
		return fmt.Sprintf("field '%s' expects type %s", unmarshalErr.Field, unmarshalErr.Type)
	}

	return "could not parse request body"
}
