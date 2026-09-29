package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/YuanJey/crypto-knights/services/trigger-service/internal/executionclient"
	"github.com/YuanJey/crypto-knights/services/trigger-service/internal/server"
	"github.com/YuanJey/crypto-knights/services/trigger-service/internal/trigger"
)

func main() {
	executionClient, err := executionclient.New(
		envOrDefault("EXECUTION_SERVICE_URL", "http://localhost:8084"),
		&http.Client{Timeout: 5 * time.Second},
	)
	if err != nil {
		slog.Error("configure execution client", "error", err)
		os.Exit(1)
	}

	address := envOrDefault("HTTP_ADDR", ":8083")
	httpServer := &http.Server{
		Addr:              address,
		Handler:           server.New(trigger.NewEngine(executionClient)).Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	shutdownContext, stop := signal.NotifyContext(
		context.Background(),
		syscall.SIGINT,
		syscall.SIGTERM,
	)
	defer stop()
	go func() {
		<-shutdownContext.Done()
		context, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := httpServer.Shutdown(context); err != nil {
			slog.Error("shutdown server", "error", err)
		}
	}()

	slog.Info("trigger service listening", "address", address)
	if err := httpServer.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		slog.Error("serve", "error", err)
		os.Exit(1)
	}
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
