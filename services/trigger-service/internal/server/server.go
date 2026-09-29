package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/YuanJey/crypto-knights/services/trigger-service/internal/trigger"
)

const maxBodyBytes int64 = 1 << 20

type Server struct {
	engine *trigger.Engine
	mux    *http.ServeMux
}

func New(engine *trigger.Engine) *Server {
	server := &Server{
		engine: engine,
		mux:    http.NewServeMux(),
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
			"service": "trigger-service",
			"status":  "ok",
		})
	})
	s.mux.HandleFunc("POST /v1/rules", s.createRule)
	s.mux.HandleFunc("GET /v1/rules", s.listRules)
	s.mux.HandleFunc("GET /v1/rules/{id}", s.getRule)
	s.mux.HandleFunc("POST /v1/rules/{id}/cancel", s.cancelRule)
	s.mux.HandleFunc("POST /v1/rules/{id}/retry", s.retryRule)
	s.mux.HandleFunc("POST /v1/ticks", s.processTick)
}

func (s *Server) createRule(w http.ResponseWriter, r *http.Request) {
	var rule trigger.Rule
	if err := decodeJSON(w, r, &rule); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	created, err := s.engine.Create(rule)
	switch {
	case errors.Is(err, trigger.ErrConflict):
		writeError(w, http.StatusConflict, err.Error())
	case err != nil:
		writeError(w, http.StatusUnprocessableEntity, err.Error())
	default:
		writeJSON(w, http.StatusCreated, created)
	}
}

func (s *Server) listRules(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"rules": s.engine.List(r.URL.Query().Get("symbol")),
	})
}

func (s *Server) getRule(w http.ResponseWriter, r *http.Request) {
	rule, err := s.engine.Get(strings.TrimSpace(r.PathValue("id")))
	if !handleEngineError(w, err) {
		writeJSON(w, http.StatusOK, rule)
	}
}

func (s *Server) cancelRule(w http.ResponseWriter, r *http.Request) {
	rule, err := s.engine.Cancel(strings.TrimSpace(r.PathValue("id")))
	if !handleEngineError(w, err) {
		writeJSON(w, http.StatusOK, rule)
	}
}

func (s *Server) retryRule(w http.ResponseWriter, r *http.Request) {
	rule, err := s.engine.Retry(r.Context(), strings.TrimSpace(r.PathValue("id")))
	if err != nil && !errors.Is(err, trigger.ErrNotFound) &&
		!errors.Is(err, trigger.ErrInvalidStatus) {
		writeJSON(w, http.StatusBadGateway, rule)
		return
	}
	if !handleEngineError(w, err) {
		writeJSON(w, http.StatusOK, rule)
	}
}

func (s *Server) processTick(w http.ResponseWriter, r *http.Request) {
	var tick trigger.Tick
	if err := decodeJSON(w, r, &tick); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	result, err := s.engine.ProcessTick(r.Context(), tick)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func handleEngineError(w http.ResponseWriter, err error) bool {
	switch {
	case err == nil:
		return false
	case errors.Is(err, trigger.ErrNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, trigger.ErrConflict):
		writeError(w, http.StatusConflict, err.Error())
	case errors.Is(err, trigger.ErrInvalidStatus):
		writeError(w, http.StatusConflict, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, err.Error())
	}
	return true
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
