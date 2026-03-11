package handler

import (
	"crypto/ecdsa"
	"net/http"
	"time"

	"golang-todo/docs"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	httpSwagger "github.com/swaggo/http-swagger"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"golang.org/x/time/rate"
)

// NewRouter builds and returns the chi router with all routes and middleware.
//
// Middleware chain (outermost → innermost):
// otelhttp → Recoverer → RequestID → StructuredLogger → Metrics → [/v1: Auth → RateLimiter] → Handler
func NewRouter(h *Handler, publicKey *ecdsa.PublicKey) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)
	r.Use(RequestIDMiddleware)
	r.Use(StructuredLoggerMiddleware)
	r.Use(MetricsMiddleware)

	// 100 requests per second burst of 100 — applied only to authenticated routes.
	limiter := rate.NewLimiter(rate.Every(time.Second), 100)

	// System routes (no auth).
	r.Get("/health", h.Health)
	r.Get("/ready", h.Ready)
	r.Handle("/metrics", promhttp.Handler())
	r.Handle("/docs/swagger.json", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(docs.SwaggerInfo.ReadDoc()))
	}))
	r.Get("/docs", http.RedirectHandler("/docs/", http.StatusMovedPermanently).ServeHTTP)
	r.Get("/docs/*", httpSwagger.Handler(
		httpSwagger.URL("/docs/swagger.json"),
	))

	// Authenticated API routes.
	r.Route("/v1", func(r chi.Router) {
		r.Use(AuthMiddleware(publicKey))
		r.Use(RateLimiterMiddleware(limiter))

		r.Get("/todos", h.ListTodos)
		r.Post("/todos", h.CreateTodo)
		r.Get("/todos/{id}", h.GetTodo)
		r.Put("/todos/{id}", h.UpdateTodo)
		r.Delete("/todos/{id}", h.DeleteTodo)
	})

	// otelhttp wraps the entire router as the outermost layer.
	return otelhttp.NewHandler(r, "todo-api")
}
