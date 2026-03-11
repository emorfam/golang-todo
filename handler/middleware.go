package handler

import (
	"context"
	"crypto/ecdsa"
	"net/http"
	"strconv"
	"strings"
	"time"

	"golang-todo/internal/apierror"
	"golang-todo/internal/logger"
	"golang-todo/metrics"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/golang-jwt/jwt/v5"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/time/rate"
)

type contextKeyUserID struct{}

// RequestIDMiddleware injects a request ID from chi into the logger context.
func RequestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := middleware.GetReqID(r.Context())
		l := logger.WithRequestID(logger.FromContext(r.Context()), reqID)
		r = r.WithContext(logger.WithContext(r.Context(), l))
		next.ServeHTTP(w, r)
	})
}

// StructuredLoggerMiddleware logs each request with duration and status using slog.
func StructuredLoggerMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		start := time.Now()

		// Enrich logger with trace ID extracted from the OTel span that otelhttp
		// created before this middleware runs.
		span := trace.SpanFromContext(r.Context())
		if span.SpanContext().IsValid() {
			traceID := span.SpanContext().TraceID().String()
			l := logger.WithTraceID(logger.FromContext(r.Context()), traceID)
			r = r.WithContext(logger.WithContext(r.Context(), l))
		}

		next.ServeHTTP(ww, r)

		logger.FromContext(r.Context()).Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", ww.Status(),
			"duration_ms", time.Since(start).Milliseconds(),
		)
	})
}

// MetricsMiddleware records Prometheus HTTP metrics.
func MetricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		start := time.Now()

		metrics.HTTPRequestsInFlight.Inc()
		defer metrics.HTTPRequestsInFlight.Dec()

		next.ServeHTTP(ww, r)

		status := strconv.Itoa(ww.Status())
		metrics.HTTPRequestsTotal.WithLabelValues(r.Method, r.URL.Path, status).Inc()
		metrics.HTTPRequestDuration.WithLabelValues(r.Method, r.URL.Path).Observe(time.Since(start).Seconds())
	})
}

// AuthMiddleware validates ES256 JWT tokens and enforces ≤15-minute lifetime.
// A nil publicKey causes every request to return 401 (key not loaded at startup).
func AuthMiddleware(publicKey *ecdsa.PublicKey) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Reject all requests when the public key was not loaded.
			if publicKey == nil {
				mapError(w, apierror.ErrUnauthorized)
				return
			}

			authHeader := r.Header.Get("Authorization")
			if !strings.HasPrefix(authHeader, "Bearer ") {
				mapError(w, apierror.ErrUnauthorized)
				return
			}
			tokenStr := strings.TrimPrefix(authHeader, "Bearer ")

			token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
				// Prevent algorithm confusion attacks.
				if t.Method != jwt.SigningMethodES256 {
					return nil, apierror.ErrUnauthorized
				}
				return publicKey, nil
			})
			if err != nil || !token.Valid {
				mapError(w, apierror.ErrUnauthorized)
				return
			}

			claims, ok := token.Claims.(jwt.MapClaims)
			if !ok {
				mapError(w, apierror.ErrUnauthorized)
				return
			}

			// Enforce maximum token lifetime of 15 minutes (service-to-service rule).
			if iat, ok := claims["iat"].(float64); ok {
				if exp, ok := claims["exp"].(float64); ok {
					lifetime := time.Duration(exp-iat) * time.Second
					if lifetime > 15*time.Minute {
						mapError(w, apierror.ErrUnauthorized)
						return
					}
				}
			}

			sub, _ := claims["sub"].(string)
			ctx := context.WithValue(r.Context(), contextKeyUserID{}, sub)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RateLimiterMiddleware enforces a global in-process rate limit.
func RateLimiterMiddleware(limiter *rate.Limiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !limiter.Allow() {
				respond(w, http.StatusTooManyRequests, map[string]string{"error": "rate limit exceeded"})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
