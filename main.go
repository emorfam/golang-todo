// @title Todo API
// @version 1.0
// @description A simple REST API for managing todo items.
// @host localhost:8080
// @BasePath /
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
package main

import (
	"context"
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"golang-todo/config"
	"golang-todo/db"
	_ "golang-todo/docs"
	todohandler "golang-todo/handler"
	"golang-todo/metrics"
	"golang-todo/repository"
	"golang-todo/service"

	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// Build-time variables (set via -ldflags).
var (
	version     = "dev"
	commit      = "none"
	buildTime   = "unknown"
	serviceName = "todo-api"
)

func main() {
	if err := run(); err != nil {
		slog.Error("server error", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// Initialize structured logger.
	initLogger(cfg)

	// Register Prometheus metrics.
	err = initPrometheus(version, commit, buildTime)
	if err != nil {
		return fmt.Errorf("setting up prometheus: %w", err)
	}

	// Open database and run migrations.
	database, err := db.Open(cfg.DBDriver, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer database.Close()

	// Initialize OTel tracer.
	tp, err := initTracer(cfg.OTLPEndpoint, serviceName)
	if err != nil {
		slog.Warn("tracing unavailable", "error", err)
	} else {
		otel.SetTracerProvider(tp)
		defer func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = tp.Shutdown(ctx)
		}()
	}

	// Load JWT public key
	var publicKey *ecdsa.PublicKey
	if cfg.JWTPublicKeyPath != "" {
		publicKey, err = loadPublicKey(cfg.JWTPublicKeyPath)
		if err != nil {
			return fmt.Errorf("loading JWT public key: %w", err)
		}
	} else {
		return fmt.Errorf("JWT_PUBLIC_KEY_PATH is required")
	}
	slog.Info("JWT public key loaded successfully")

	// Wire layers.
	todoRepo := repository.NewTodoRepository(database, cfg.DBDriver)
	todoSvc := service.NewTodoService(todoRepo)
	h := todohandler.New(todoSvc, database)
	router := todohandler.NewRouter(h, publicKey)

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	slog.Info("starting server", "addr", srv.Addr, "env", cfg.Env)

	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("listen error", "error", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down server")
	shutCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutCtx); err != nil {
		return fmt.Errorf("graceful shutdown: %w", err)
	}
	slog.Info("server stopped")
	return nil
}

func initLogger(cfg *config.Config) {
	var handler slog.Handler
	opts := &slog.HandlerOptions{Level: slog.LevelInfo}
	if cfg.LogFormat == "json" {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}
	logger := slog.New(handler)
	slog.SetDefault(logger)
}

func initPrometheus(version, commit, buildTime string) error {
	if err := metrics.Register(prometheus.DefaultRegisterer); err != nil {
		return fmt.Errorf("registering metrics: %w", err)
	}
	metrics.BuildInfo.WithLabelValues(version, commit, buildTime).Set(1)
	return nil
}

func initTracer(endpoint, svcName string) (*sdktrace.TracerProvider, error) {
	// The gRPC exporter expects a bare host:port target, not a URL with a scheme.
	// Strip http:// or https:// if present so that values like
	// "http://localhost:4317" (common in .env files) work correctly.
	endpoint = strings.TrimPrefix(endpoint, "https://")
	endpoint = strings.TrimPrefix(endpoint, "http://")

	expCtx, expCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer expCancel()
	exp, err := otlptracegrpc.New(expCtx,
		otlptracegrpc.WithEndpoint(endpoint),
		otlptracegrpc.WithInsecure(),
	)
	if err != nil {
		return nil, fmt.Errorf("creating OTLP exporter: %w", err)
	}

	resCtx, resCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer resCancel()
	res, err := resource.New(resCtx,
		resource.WithAttributes(semconv.ServiceName(svcName)),
	)
	if err != nil {
		return nil, fmt.Errorf("creating resource: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(res),
	)
	return tp, nil
}

func loadPublicKey(path string) (*ecdsa.PublicKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading public key file: %w", err)
	}

	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block from %s", path)
	}

	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parsing public key: %w", err)
	}

	ecKey, ok := pub.(*ecdsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("public key is not an ECDSA key")
	}
	return ecKey, nil
}
