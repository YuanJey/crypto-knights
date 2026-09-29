package news

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestManagerRefreshesOnlyDueSources(t *testing.T) {
	var requests atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		w.Header().Set("Content-Type", "application/rss+xml")
		fmt.Fprint(w, `<?xml version="1.0"?>
<rss version="2.0"><channel><item>
  <title>Test event</title>
  <link>https://example.com/event</link>
  <pubDate>Tue, 29 Sep 2026 12:00:00 GMT</pubDate>
</item></channel></rss>`)
	}))
	defer server.Close()

	fetcher, err := NewFetcher(server.Client(), "test-agent")
	if err != nil {
		t.Fatalf("new fetcher: %v", err)
	}
	now := time.Date(2026, 9, 29, 12, 1, 0, 0, time.UTC)
	fetcher.now = func() time.Time { return now }
	store := NewStore(100, 72*time.Hour)
	store.now = func() time.Time { return now }
	manager := NewManager(store, fetcher, []Source{{
		ID:                  "official",
		Name:                "Official",
		Kind:                SourceKindRSS,
		Tier:                "A",
		SourceType:          "official",
		URL:                 server.URL,
		PollIntervalSeconds: 60,
		MaxItems:            10,
		Enabled:             true,
	}})
	manager.now = func() time.Time { return now }

	first := manager.RefreshDue(context.Background())
	if len(first) != 1 || first[0].Status != "healthy" {
		t.Fatalf("first refresh = %#v", first)
	}
	second := manager.RefreshDue(context.Background())
	if len(second) != 0 {
		t.Fatalf("second refresh should be rate limited: %#v", second)
	}
	if requests.Load() != 1 {
		t.Fatalf("requests = %d, want 1", requests.Load())
	}

	now = now.Add(61 * time.Second)
	third := manager.RefreshDue(context.Background())
	if len(third) != 1 || requests.Load() != 2 {
		t.Fatalf("third refresh = %#v, requests = %d", third, requests.Load())
	}
	if len(manager.Articles(ListFilter{Limit: 10})) != 1 {
		t.Fatal("expected one deduplicated article")
	}
}
