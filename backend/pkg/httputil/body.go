package httputil

import (
	"encoding/json"
	"errors"
	"net/http"
)

// DefaultJSONBodyLimit is the maximum accepted size for JSON request bodies.
// Handlers that legitimately receive larger payloads (zip drops) pass their
// own limit to DecodeJSONBody.
const DefaultJSONBodyLimit int64 = 1 << 20 // 1 MiB

// ErrBodyTooLarge is returned by DecodeJSONBody when the request body exceeds
// the configured limit. Callers map it to HTTP 413.
var ErrBodyTooLarge = errors.New("request body too large")

// DecodeJSONBody decodes a JSON request body enforcing a size limit. A limit
// of 0 applies DefaultJSONBodyLimit. Oversized bodies return ErrBodyTooLarge.
func DecodeJSONBody(w http.ResponseWriter, r *http.Request, target interface{}, limit int64) error {
	if limit <= 0 {
		limit = DefaultJSONBodyLimit
	}
	if r.Body == nil {
		return nil
	}
	r.Body = http.MaxBytesReader(w, r.Body, limit)
	err := json.NewDecoder(r.Body).Decode(target)
	var maxErr *http.MaxBytesError
	if errors.As(err, &maxErr) {
		return ErrBodyTooLarge
	}
	return err
}

// WriteDecodeError writes the canonical error response for a decode failure:
// 413 for oversized bodies, 400 otherwise.
func WriteDecodeError(w http.ResponseWriter, err error) {
	if errors.Is(err, ErrBodyTooLarge) {
		WriteError(w, http.StatusRequestEntityTooLarge, "Request body too large")
		return
	}
	WriteError(w, http.StatusBadRequest, "Invalid request payload: "+err.Error())
}
