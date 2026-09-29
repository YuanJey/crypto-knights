package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/YuanJey/crypto-knights/services/execution-service/internal/execution"
)

const maxBodyBytes int64 = 1 << 20

type Server struct {
	executor *execution.Executor
	mux      *http.ServeMux
}

func New(executor *execution.Executor) *Server {
	server := &Server{
		executor: executor,
		mux:      http.NewServeMux(),
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
			"service": "execution-service",
			"status":  "ok",
		})
	})
	s.mux.HandleFunc("POST /v1/executions", s.createExecution)
	s.mux.HandleFunc("GET /v1/executions", s.listExecutions)
	s.mux.HandleFunc("GET /v1/executions/{id}", s.getExecution)
}

func (s *Server) createExecution(w http.ResponseWriter, r *http.Request) {
	var request execution.Request
	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	result, err := s.executor.Submit(r.Context(), request)
	switch {
	case errors.Is(err, execution.ErrIdempotencyConflict):
		writeError(w, http.StatusConflict, err.Error())
	case err != nil:
		writeError(w, http.StatusUnprocessableEntity, err.Error())
	case result.Record.Status == "failed":
		writeJSON(w, http.StatusBadGateway, result.Record)
	case result.Created:
		writeJSON(w, http.StatusCreated, result.Record)
	default:
		writeJSON(w, http.StatusOK, result.Record)
	}
}

func (s *Server) listExecutions(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"executions": s.executor.List(),
	})
}

func (s *Server) getExecution(w http.ResponseWriter, r *http.Request) {
	record, err := s.executor.Get(strings.TrimSpace(r.PathValue("id")))
	if errors.Is(err, execution.ErrNotFound) {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, record)
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
