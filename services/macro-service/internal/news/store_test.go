package news

import (
	"testing"
	"time"
)

func TestStoreDeduplicatesURLAndPrefersHigherTier(t *testing.T) {
	store := NewStore(100, 72*time.Hour)
	now := time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC)
	store.now = func() time.Time { return now }

	discoveryTime := now.Add(-time.Minute)
	added := store.Upsert([]Article{{
		SourceID:    "google",
		SourceName:  "Google News",
		Publisher:   "Example",
		Tier:        "D",
		SourceType:  "aggregator",
		Title:       "Policy decision reported",
		URL:         "https://EXAMPLE.com/policy?id=1&utm_source=google#top",
		PublishedAt: &discoveryTime,
		RetrievedAt: now,
	}})
	if added != 1 {
		t.Fatalf("added = %d, want 1", added)
	}

	officialTime := now.Add(-2 * time.Minute)
	added = store.Upsert([]Article{{
		SourceID:    "official",
		SourceName:  "Example Authority",
		Publisher:   "Example Authority",
		Tier:        "A",
		SourceType:  "official",
		Title:       "Policy decision",
		URL:         "https://example.com/policy?id=1",
		PublishedAt: &officialTime,
		RetrievedAt: now,
	}})
	if added != 0 {
		t.Fatalf("added duplicate = %d, want 0", added)
	}

	articles := store.List(ListFilter{Limit: 10})
	if len(articles) != 1 {
		t.Fatalf("article count = %d, want 1", len(articles))
	}
	if articles[0].Tier != "A" || articles[0].SourceID != "official" {
		t.Fatalf("higher-tier source did not replace discovery source: %#v", articles[0])
	}
	if articles[0].URL != "https://example.com/policy?id=1" {
		t.Fatalf("canonical URL = %q", articles[0].URL)
	}
}

func TestStoreAppliesRetentionAndFilters(t *testing.T) {
	store := NewStore(10, 24*time.Hour)
	now := time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC)
	store.now = func() time.Time { return now }
	recent := now.Add(-time.Hour)
	stale := now.Add(-48 * time.Hour)

	store.Upsert([]Article{
		{
			SourceID:    "official",
			SourceName:  "Official",
			Publisher:   "Official",
			Tier:        "A",
			SourceType:  "official",
			Title:       "Recent",
			URL:         "https://example.com/recent",
			PublishedAt: &recent,
			RetrievedAt: now,
		},
		{
			SourceID:    "wire",
			SourceName:  "Wire",
			Publisher:   "Wire",
			Tier:        "B",
			SourceType:  "mainstream_press",
			Title:       "Stale",
			URL:         "https://example.com/stale",
			PublishedAt: &stale,
			RetrievedAt: now,
		},
	})

	articles := store.List(ListFilter{Limit: 10})
	if len(articles) != 1 || articles[0].Title != "Recent" {
		t.Fatalf("unexpected retained articles: %#v", articles)
	}
}
