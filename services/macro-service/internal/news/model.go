package news

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
)

type SourceKind string

const (
	SourceKindRSS   SourceKind = "rss"
	SourceKindGDELT SourceKind = "gdelt"
)

type Source struct {
	ID                  string     `json:"id"`
	Name                string     `json:"name"`
	Kind                SourceKind `json:"kind"`
	Tier                string     `json:"tier"`
	SourceType          string     `json:"source_type"`
	URL                 string     `json:"url"`
	Query               string     `json:"query,omitempty"`
	Language            string     `json:"language,omitempty"`
	Country             string     `json:"country,omitempty"`
	PollIntervalSeconds int64      `json:"poll_interval_seconds"`
	MaxItems            int        `json:"max_items"`
	Enabled             bool       `json:"enabled"`
}

func (s Source) Validate() error {
	if strings.TrimSpace(s.ID) == "" || strings.TrimSpace(s.Name) == "" {
		return errors.New("source id and name are required")
	}
	if s.Kind != SourceKindRSS && s.Kind != SourceKindGDELT {
		return fmt.Errorf("source %s has unsupported kind %q", s.ID, s.Kind)
	}
	if !oneOf(s.Tier, "A", "B", "C", "D") {
		return fmt.Errorf("source %s has invalid tier %q", s.ID, s.Tier)
	}
	if strings.TrimSpace(s.SourceType) == "" {
		return fmt.Errorf("source %s requires source_type", s.ID)
	}
	parsedURL, err := url.Parse(s.URL)
	if err != nil || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") ||
		parsedURL.Host == "" {
		return fmt.Errorf("source %s has invalid URL", s.ID)
	}
	if s.Kind == SourceKindGDELT && strings.TrimSpace(s.Query) == "" {
		return fmt.Errorf("GDELT source %s requires query", s.ID)
	}
	if s.PollIntervalSeconds < 10 {
		return fmt.Errorf("source %s poll interval must be at least 10 seconds", s.ID)
	}
	if s.Kind == SourceKindGDELT && s.PollIntervalSeconds < 300 {
		return fmt.Errorf("GDELT source %s poll interval must be at least 300 seconds", s.ID)
	}
	if s.MaxItems < 1 || s.MaxItems > 250 {
		return fmt.Errorf("source %s max_items must be between 1 and 250", s.ID)
	}
	return nil
}

func (s Source) PollInterval() time.Duration {
	return time.Duration(s.PollIntervalSeconds) * time.Second
}

type Article struct {
	ID          string     `json:"id"`
	SourceID    string     `json:"source_id"`
	SourceName  string     `json:"source_name"`
	Publisher   string     `json:"publisher"`
	Tier        string     `json:"tier"`
	SourceType  string     `json:"source_type"`
	Title       string     `json:"title"`
	URL         string     `json:"url"`
	Language    string     `json:"language,omitempty"`
	Country     string     `json:"country,omitempty"`
	PublishedAt *time.Time `json:"published_at"`
	RetrievedAt time.Time  `json:"retrieved_at"`
}

type SourceState struct {
	Source        Source     `json:"source"`
	Status        string     `json:"status"`
	LastAttemptAt *time.Time `json:"last_attempt_at"`
	LastSuccessAt *time.Time `json:"last_success_at"`
	NextAttemptAt *time.Time `json:"next_attempt_at"`
	LastError     string     `json:"last_error,omitempty"`
	LastItemCount int        `json:"last_item_count"`
	TotalItems    int64      `json:"total_items"`
}

type ListFilter struct {
	SourceID string
	Tier     string
	Since    *time.Time
	Limit    int
}

func oneOf(value string, allowed ...string) bool {
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}
