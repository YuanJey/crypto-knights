package server

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/YuanJey/crypto-knights/services/macro-service/internal/news"
	"github.com/YuanJey/crypto-knights/services/macro-service/internal/report"
)

const maxReportBytes int64 = 4 << 20

type Server struct {
	store       *report.Store
	newsManager *news.Manager
	mux         *http.ServeMux
}

func New(store *report.Store, newsManager *news.Manager) *Server {
	server := &Server{
		store:       store,
		newsManager: newsManager,
		mux:         http.NewServeMux(),
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
			"service": "macro-service",
			"status":  "ok",
		})
	})
	s.mux.HandleFunc("POST /v1/reports", s.createReport)
	s.mux.HandleFunc("GET /v1/reports/latest", s.latestReport)
	s.mux.HandleFunc("GET /v1/reports/{id}", s.getReport)
	s.mux.HandleFunc("GET /v1/signals/latest", s.latestSignals)
	s.mux.HandleFunc("GET /v1/news", s.listNews)
	s.mux.HandleFunc("GET /v1/news/sources", s.listNewsSources)
	s.mux.HandleFunc("POST /v1/news/refresh", s.refreshNews)
}

func (s *Server) createReport(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxReportBytes)
	document, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "read report: "+err.Error())
		return
	}

	record, err := s.store.Create(document)
	switch {
	case errors.Is(err, report.ErrConflict):
		writeError(w, http.StatusConflict, err.Error())
	case err != nil:
		writeError(w, http.StatusUnprocessableEntity, err.Error())
	default:
		writeJSON(w, http.StatusCreated, record)
	}
}

func (s *Server) latestReport(w http.ResponseWriter, r *http.Request) {
	record, err := s.store.Latest(r.URL.Query().Get("asset"))
	if errors.Is(err, report.ErrNotFound) {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, record)
}

func (s *Server) getReport(w http.ResponseWriter, r *http.Request) {
	record, err := s.store.Get(strings.TrimSpace(r.PathValue("id")))
	if errors.Is(err, report.ErrNotFound) {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, record)
}

func (s *Server) latestSignals(w http.ResponseWriter, r *http.Request) {
	signals, err := s.store.LatestSignals(
		r.URL.Query().Get("asset"),
		r.URL.Query().Get("horizon"),
	)
	if errors.Is(err, report.ErrNotFound) {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"signals": signals})
}

func (s *Server) listNews(w http.ResponseWriter, r *http.Request) {
	limit := 100
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > 500 {
			writeError(w, http.StatusBadRequest, "limit must be between 1 and 500")
			return
		}
		limit = parsed
	}

	var since *time.Time
	if raw := strings.TrimSpace(r.URL.Query().Get("since")); raw != "" {
		parsed, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			writeError(w, http.StatusBadRequest, "since must be an RFC3339 timestamp")
			return
		}
		parsed = parsed.UTC()
		since = &parsed
	}

	tier := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("tier")))
	if tier != "" && tier != "A" && tier != "B" && tier != "C" && tier != "D" {
		writeError(w, http.StatusBadRequest, "tier must be A, B, C, or D")
		return
	}
	articles := s.newsManager.Articles(news.ListFilter{
		SourceID: strings.ToLower(strings.TrimSpace(r.URL.Query().Get("source_id"))),
		Tier:     tier,
		Since:    since,
		Limit:    limit,
	})
	writeJSON(w, http.StatusOK, map[string]any{
		"count": len(articles),
		"items": articles,
	})
}

func (s *Server) listNewsSources(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"sources": s.newsManager.Sources(),
	})
}

func (s *Server) refreshNews(w http.ResponseWriter, r *http.Request) {
	refreshed := s.newsManager.RefreshDue(r.Context())
	writeJSON(w, http.StatusOK, map[string]any{
		"refreshed": refreshed,
	})
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
