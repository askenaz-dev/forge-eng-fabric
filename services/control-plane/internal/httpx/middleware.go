package httpx

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"
)

type ctxKey int

const correlationKey ctxKey = 1

// CorrelationID propagates an X-Correlation-Id header (or generates a new one)
// and stores it in the request context.
func CorrelationID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Correlation-Id")
		if id == "" {
			id = uuid.NewString()
		}
		w.Header().Set("X-Correlation-Id", id)
		ctx := context.WithValue(r.Context(), correlationKey, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// CorrelationFromContext returns the correlation id, if set.
func CorrelationFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(correlationKey).(string); ok {
		return v
	}
	return ""
}

// AccessLog logs one line per request.
func AccessLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: 200}
		next.ServeHTTP(sw, r)
		log.Printf("%s %s -> %d (%s) cid=%s",
			r.Method, r.URL.Path, sw.status, time.Since(start),
			CorrelationFromContext(r.Context()))
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}
