package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"learning-serbian-platform/backend/internal/apperr"
)

// errorBody is the error envelope documented in api/openapi.yaml.
type errorBody struct {
	Error errorDetail `json:"error"`
}

type errorDetail struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		slog.Error("write json response", "error", err)
	}
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, errorBody{Error: errorDetail{Code: code, Message: message}})
}

var statusByKind = map[apperr.Kind]int{
	apperr.Invalid:         http.StatusUnprocessableEntity,
	apperr.Unauthenticated: http.StatusUnauthorized,
	apperr.Conflict:        http.StatusConflict,
	apperr.NotFound:        http.StatusNotFound,
}

// writeServiceError is the single place that maps service errors to HTTP.
// Unexpected errors are logged and answered with a generic 500.
func writeServiceError(w http.ResponseWriter, r *http.Request, err error) {
	var appErr *apperr.Error
	if errors.As(err, &appErr) {
		if status, ok := statusByKind[appErr.Kind]; ok {
			writeJSON(w, status, errorBody{Error: errorDetail{Code: appErr.Code, Message: appErr.Message, Fields: appErr.Fields}})
			return
		}
	}
	slog.ErrorContext(r.Context(), "unhandled error", "method", r.Method, "path", r.URL.Path, "error", err)
	writeError(w, http.StatusInternalServerError, "internal_error", "Something went wrong on our side. Try again.")
}

const maxJSONBody = 64 << 10 // 64 KiB is plenty for any form in this app

// decodeJSON reads exactly one JSON object into dst, rejecting unknown
// fields and oversized bodies.
func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	if ct := r.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		return apperr.New(apperr.Invalid, "unsupported_content_type", "Send the request body as application/json.")
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxJSONBody))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return apperr.New(apperr.Invalid, "invalid_json", "The request body is not valid JSON for this endpoint.")
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return apperr.New(apperr.Invalid, "invalid_json", "The request body must contain a single JSON object.")
	}
	return nil
}
