package request

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"microservice/pkg/apperror"
)

// MaxBodyBytes caps the size of a JSON request body to guard against unbounded
// uploads. 1 MiB is generous for the small payloads these endpoints accept.
const MaxBodyBytes = 1 << 20

// DecodeJSON wraps r.Body with http.MaxBytesReader before decoding into dst so
// that oversized payloads are rejected instead of being read into memory.
// Unknown fields are rejected so that client typos fail loudly instead of
// being silently dropped. It returns an *apperror.AppError on failure, ready
// to hand to response.Error.
func DecodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, MaxBodyBytes)

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(dst); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			return apperror.InvalidInput("request body too large")
		}
		// DisallowUnknownFields surfaces an untyped error of the form
		// `json: unknown field "foo"`; surface the field name to the client.
		if field, ok := strings.CutPrefix(err.Error(), "json: unknown field "); ok {
			return apperror.InvalidInput("unknown field " + field)
		}
		return apperror.InvalidInput("invalid request body")
	}

	// Reject trailing data so a body with more than one JSON value is not
	// silently accepted.
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return apperror.InvalidInput("request body must contain a single JSON object")
	}

	return nil
}
