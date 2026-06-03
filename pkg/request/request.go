package request

import (
	"encoding/json"
	"errors"
	"net/http"

	"microservice/pkg/appError"
)

// MaxBodyBytes caps the size of a JSON request body to guard against unbounded
// uploads. 1 MiB is generous for the small payloads these endpoints accept.
const MaxBodyBytes = 1 << 20

// DecodeJSON wraps r.Body with http.MaxBytesReader before decoding into dst so
// that oversized payloads are rejected instead of being read into memory. It
// returns an *appError.AppError on failure, ready to hand to response.Error.
func DecodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, MaxBodyBytes)

	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			return appError.InvalidInput("request body too large")
		}
		return appError.InvalidInput("invalid request body")
	}
	return nil
}
