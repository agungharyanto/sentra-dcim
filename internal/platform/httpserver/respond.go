package httpserver

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5/middleware"
)

// ErrorBody is the standard API error envelope.
type ErrorBody struct {
	Error ErrorDetail `json:"error"`
}

// ErrorDetail describes an API error.
type ErrorDetail struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id,omitempty"`
}

// JSON writes v as a JSON response with the given status code.
func JSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

// Error writes a standard error envelope.
func Error(w http.ResponseWriter, r *http.Request, code int, errCode, msg string) {
	JSON(w, code, ErrorBody{Error: ErrorDetail{Code: errCode, Message: msg, RequestID: middleware.GetReqID(r.Context())}})
}
