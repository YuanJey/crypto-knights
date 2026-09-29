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

	"github.com/shopspring/decimal"

	"github.com/YuanJey/crypto-knights/services/execution-service/internal/exchange/paper"
	"github.com/YuanJey/crypto-knights/services/execution-service/internal/execution"
	"github.com/YuanJey/crypto-knights/services/execution-service/internal/server"
)

func main() {
	if mode := envOrDefault("EXCHANGE_MODE", "paper"); mode != "paper" {
		slog.Error("unsupported EXCHANGE_MODE; live trading is not configured", "mode", mode)
		os.Exit(1)
	}
	allowedSymbols, err := execution.ParseAllowedSymbols(
		envOrDefault("ALLOWED_SYMBOLS", "BTCUSDT,ETHUSDT"),
	)
	if err != nil {
		slog.Error("configure allowed symbols", "error", err)
		os.Exit(1)
	}
	maxOrderNotional, err := decimal.NewFromString(
		envOrDefault("MAX_ORDER_NOTIONAL", "10000"),
	)
	if err != nil {
		slog.Error("configure maximum order notional", "error", err)
		os.Exit(1)
	}
	maxTriggerAgeSeconds, err := strconv.ParseInt(
		envOrDefault("MAX_TRIGGER_AGE_SECONDS", "30"),
		10,
		64,
	)
	if err != nil {
		slog.Error("configure maximum trigger age", "error", err)
		os.Exit(1)
	}

	executor, err := execution.NewExecutor(paper.New(), execution.Policy{
		AllowedSymbols:   allowedSymbols,
		MaxOrderNotional: maxOrderNotional,
		MaxTriggerAge:    time.Duration(maxTriggerAgeSeconds) * time.Second,
		MaxFutureSkew:    5 * time.Second,
	})
	if err != nil {
		slog.Error("create executor", "error", err)
		os.Exit(1)
	}

	address := envOrDefault("HTTP_ADDR", ":8084")
	httpServer := &http.Server{
		Addr:              address,
		Handler:           server.New(executor).Handler(),
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

	slog.Info("execution service listening",
		"address", address,
		"exchange_mode", "paper",
		"maximum_order_notional", maxOrderNotional,
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
