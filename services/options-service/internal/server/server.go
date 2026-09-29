package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/YuanJey/crypto-knights/services/options-service/internal/options"
)

const maxBodyBytes int64 = 1 << 20

type Server struct {
	store *options.Store
	mux   *http.ServeMux
}

func New(store *options.Store) *Server {
	server := &Server{
		store: store,
		mux:   http.NewServeMux(),
	}
	server.routes()
	return server
}

func (s *Server) Handler() http.Handler {
	return recoverPanic(logRequests(s.mux))
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{
			"service": "options-service",
			"status":  "ok",
		})
	})
	s.mux.HandleFunc("POST /v1/trades", s.createTrade)
	s.mux.HandleFunc("GET /v1/trades/{id}", s.getTrade)
	s.mux.HandleFunc("GET /v1/signals", s.getSignal)
}

func (s *Server) createTrade(w http.ResponseWriter, r *http.Request) {
	var trade options.Trade
	if err := decodeJSON(w, r, &trade); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	created, err := s.store.Create(trade)
	switch {
	case errors.Is(err, options.ErrConflict):
		writeError(w, http.StatusConflict, err.Error())
	case err != nil:
		writeError(w, http.StatusUnprocessableEntity, err.Error())
	default:
		writeJSON(w, http.StatusCreated, created)
	}
}

func (s *Server) getTrade(w http.ResponseWriter, r *http.Request) {
	trade, err := s.store.Get(strings.TrimSpace(r.PathValue("id")))
	if errors.Is(err, options.ErrNotFound) {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, trade)
}

func (s *Server) getSignal(w http.ResponseWriter, r *http.Request) {
	windowSeconds := int64(3600)
	if raw := r.URL.Query().Get("window_seconds"); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || parsed <= 0 || parsed > 30*24*60*60 {
			writeError(w, http.StatusBadRequest, "window_seconds must be between 1 and 2592000")
			return
		}
		windowSeconds = parsed
	}

	signal, err := s.store.CalculateSignal(
		r.URL.Query().Get("underlying"),
		time.Duration(windowSeconds)*time.Second,
	)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, signal)
}

func decodeJSON(w http.ResponseWriter, r *http.Request, destination any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return fmt.Errorf("decode JSON: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("request body must contain one JSON value")
		}
		return fmt.Errorf("decode trailing JSON: %w", err)
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		slog.Error("write response", "error", err)
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		slog.Info("request", "method", r.Method, "path", r.URL.Path)
		next.ServeHTTP(w, r)
	})
}

func recoverPanic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				slog.Error("panic serving request", "panic", recovered)
				writeError(w, http.StatusInternalServerError, "internal server error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}
