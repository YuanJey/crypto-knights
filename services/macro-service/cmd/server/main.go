package main

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/YuanJey/crypto-knights/services/macro-service/internal/news"
	"github.com/YuanJey/crypto-knights/services/macro-service/internal/report"
	"github.com/YuanJey/crypto-knights/services/macro-service/internal/server"
)

func main() {
	sources, err := news.LoadSources(os.Getenv("NEWS_SOURCES_FILE"))
	if err != nil {
		slog.Error("load news sources", "error", err)
		os.Exit(1)
	}
	maxNewsItems, err := positiveIntEnv("NEWS_MAX_ITEMS", 5000)
	if err != nil {
		slog.Error("configure news retention", "error", err)
		os.Exit(1)
	}
	retentionHours, err := positiveIntEnv("NEWS_RETENTION_HOURS", 72)
	if err != nil {
		slog.Error("configure news retention", "error", err)
		os.Exit(1)
	}
	newsFetcher, err := news.NewFetcher(
		&http.Client{
			Timeout: 12 * time.Second,
			Transport: &http.Transport{
				Proxy:                 http.ProxyFromEnvironment,
				DialContext:           (&net.Dialer{Timeout: 5 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
				ForceAttemptHTTP2:     true,
				MaxIdleConns:          20,
				IdleConnTimeout:       90 * time.Second,
				TLSHandshakeTimeout:   5 * time.Second,
				ResponseHeaderTimeout: 8 * time.Second,
				ExpectContinueTimeout: time.Second,
			},
		},
		envOrDefault(
			"NEWS_USER_AGENT",
			"crypto-knights/0.1 (+https://github.com/YuanJey/crypto-knights)",
		),
	)
	if err != nil {
		slog.Error("create news fetcher", "error", err)
		os.Exit(1)
	}
	newsManager := news.NewManager(
		news.NewStore(maxNewsItems, time.Duration(retentionHours)*time.Hour),
		newsFetcher,
		sources,
	)

	shutdownContext, stop := signal.NotifyContext(
		context.Background(),
		syscall.SIGINT,
		syscall.SIGTERM,
	)
	defer stop()
	newsManager.Start(shutdownContext)

	address := envOrDefault("HTTP_ADDR", ":8081")
	handler := server.New(report.NewStore(), newsManager).Handler()
	httpServer := &http.Server{
		Addr:              address,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		<-shutdownContext.Done()
		context, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := httpServer.Shutdown(context); err != nil {
			slog.Error("shutdown server", "error", err)
		}
	}()

	slog.Info("macro service listening", "address", address)
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

func positiveIntEnv(key string, fallback int) (int, error) {
	raw := envOrDefault(key, strconv.Itoa(fallback))
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return 0, errors.New(key + " must be a positive integer")
	}
	return value, nil
}
