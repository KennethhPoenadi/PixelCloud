// Package httpx holds small HTTP helpers shared by handlers and middleware.
package httpx

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/KennethhPoenadi/PixelCloud/api/internal/apierr"
)

type ctxKey int

const (
	requestIDKey ctxKey = iota
	loggerKey
)

func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey, id)
}

func RequestID(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey).(string)
	return id
}

func WithLogger(ctx context.Context, l *slog.Logger) context.Context {
	return context.WithValue(ctx, loggerKey, l)
}

// Logger returns the request-scoped logger (carries node_id, request_id, user_id).
func Logger(ctx context.Context) *slog.Logger {
	if l, ok := ctx.Value(loggerKey).(*slog.Logger); ok {
		return l
	}
	return slog.Default()
}

func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if v == nil {
		return
	}
	_ = json.NewEncoder(w).Encode(v)
}

type errorBody struct {
	Error errorDetail `json:"error"`
}

type errorDetail struct {
	Code    apierr.Code `json:"code"`
	Message string      `json:"message"`
}

// WriteError renders err as the standard error envelope. Errors that are not
// *apierr.Error are logged and hidden behind a generic INTERNAL response.
func WriteError(w http.ResponseWriter, r *http.Request, err error) {
	var ae *apierr.Error
	if !errors.As(err, &ae) {
		Logger(r.Context()).Error("request failed", "err", err)
		ae = apierr.New(apierr.Internal, "something went wrong, please try again")
	}
	WriteJSON(w, ae.Status(), errorBody{Error: errorDetail{Code: ae.Code, Message: ae.Message}})
}

// DecodeJSON strictly decodes a JSON request body (unknown fields rejected).
func DecodeJSON(r *http.Request, dst any) error {
	dec := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return apierr.New(apierr.Validation, "invalid JSON body: %v", err)
	}
	if dec.More() {
		return apierr.New(apierr.Validation, "invalid JSON body: trailing data")
	}
	return nil
}
