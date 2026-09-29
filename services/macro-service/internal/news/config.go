package news

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
)

//go:embed default-sources.json
var defaultSourcesDocument []byte

type sourceConfig struct {
	Sources []Source `json:"sources"`
}

func LoadSources(path string) ([]Source, error) {
	document := defaultSourcesDocument
	if strings.TrimSpace(path) != "" {
		fileDocument, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read news sources file: %w", err)
		}
		document = fileDocument
	}

	decoder := json.NewDecoder(bytes.NewReader(document))
	decoder.DisallowUnknownFields()
	var config sourceConfig
	if err := decoder.Decode(&config); err != nil {
		return nil, fmt.Errorf("decode news sources: %w", err)
	}
	if len(config.Sources) == 0 {
		return nil, errors.New("at least one news source is required")
	}

	seen := make(map[string]struct{}, len(config.Sources))
	for index := range config.Sources {
		source := &config.Sources[index]
		source.ID = strings.ToLower(strings.TrimSpace(source.ID))
		source.Name = strings.TrimSpace(source.Name)
		source.Tier = strings.ToUpper(strings.TrimSpace(source.Tier))
		source.SourceType = strings.ToLower(strings.TrimSpace(source.SourceType))
		source.URL = strings.TrimSpace(source.URL)
		source.Query = strings.TrimSpace(source.Query)
		source.Language = strings.ToLower(strings.TrimSpace(source.Language))
		source.Country = strings.ToUpper(strings.TrimSpace(source.Country))

		if err := source.Validate(); err != nil {
			return nil, err
		}
		if _, exists := seen[source.ID]; exists {
			return nil, fmt.Errorf("duplicate news source id %q", source.ID)
		}
		seen[source.ID] = struct{}{}
	}
	return config.Sources, nil
}
