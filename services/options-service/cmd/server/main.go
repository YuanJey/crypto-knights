package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/YuanJey/crypto-knights/services/options-service/internal/options"
	"github.com/YuanJey/crypto-knights/services/options-service/internal/server"
)

func main() {
	minimumNotional, err := options.ParseMinimumNotional(
		envOrDefault("MIN_PREMIUM_NOTIONAL", "100000"),
	)
	if err != nil {
		slog.Error("invalid MIN_PREMIUM_NOTIONAL", "error", err)
		os.Exit(1)
	}
	maximumKeyLevels, err := strconv.Atoi(envOrDefault("MAX_KEY_LEVELS", "10"))
	if err != nil {
		slog.Error("invalid MAX_KEY_LEVELS", "error", err)
		os.Exit(1)
	}
	store, err := options.NewStore(minimumNotional, maximumKeyLevels)
	if err != nil {
		slog.Error("create store", "error", err)
		os.Exit(1)
	}

	address := envOrDefault("HTTP_ADDR", ":8082")
	httpServer := &http.Server{
		Addr:              address,
		Handler:           server.New(store).Handler(),
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

	slog.Info("options service listening",
		"address", address,
		"minimum_premium_notional", minimumNotional,
		"maximum_key_levels", maximumKeyLevels,
	)
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
