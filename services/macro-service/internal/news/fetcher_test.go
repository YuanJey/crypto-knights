package news

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestFetcherParsesRSSAndResolvesPublisher(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("User-Agent"); got != "test-agent" {
			t.Errorf("user agent = %q", got)
		}
		w.Header().Set("Content-Type", "application/rss+xml")
		fmt.Fprint(w, `<?xml version="1.0" encoding="us-ascii"?>
<rss version="2.0">
  <channel>
    <item>
      <title>Central bank announces decision</title>
      <link>/story?id=1&amp;utm_source=test</link>
      <pubDate><![CDATA[Tue, 29 Sep 2026 12:00:00 GMT]]></pubDate>
      <source url="https://publisher.example">Example Wire</source>
    </item>
  </channel>
</rss>`)
	}))
	defer server.Close()

	fetcher, err := NewFetcher(server.Client(), "test-agent")
	if err != nil {
		t.Fatalf("new fetcher: %v", err)
	}
	fetcher.now = func() time.Time {
		return time.Date(2026, 9, 29, 12, 1, 0, 0, time.UTC)
	}
	articles, err := fetcher.Fetch(context.Background(), Source{
		ID:         "test-rss",
		Name:       "Test RSS",
		Kind:       SourceKindRSS,
		Tier:       "B",
		SourceType: "mainstream_press",
		URL:        server.URL + "/rss",
		Language:   "en",
		MaxItems:   10,
	})
	if err != nil {
		t.Fatalf("fetch RSS: %v", err)
	}
	if len(articles) != 1 {
		t.Fatalf("article count = %d, want 1", len(articles))
	}
	article := articles[0]
	if article.Publisher != "Example Wire" {
		t.Fatalf("publisher = %q", article.Publisher)
	}
	if article.URL != server.URL+"/story?id=1&utm_source=test" {
		t.Fatalf("URL = %q", article.URL)
	}
	if article.PublishedAt == nil ||
		!article.PublishedAt.Equal(time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC)) {
		t.Fatalf("published_at = %v", article.PublishedAt)
	}
}

func TestParsePublishedAtSupportsOfficialFeedFormats(t *testing.T) {
	cases := map[string]time.Time{
		"Fri, 25 Sep 2026 20:30:00 GMT": time.Date(2026, 9, 25, 20, 30, 0, 0, time.UTC),
		"Thu, 24 Sep 2026 08:30:00 EDT": time.Date(2026, 9, 24, 12, 30, 0, 0, time.UTC),
		"20260929T120000Z":              time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC),
	}
	for value, expected := range cases {
		parsed := parsePublishedAt(value)
		if parsed == nil || !parsed.Equal(expected) {
			t.Errorf("parse %q = %v, want %v", value, parsed, expected)
		}
	}
}

func TestFetcherBuildsAndParsesGDELTRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("query") != "sanctions OR tariff" {
			t.Errorf("query = %q", r.URL.Query().Get("query"))
		}
		if r.URL.Query().Get("format") != "json" ||
			r.URL.Query().Get("mode") != "artlist" ||
			r.URL.Query().Get("maxrecords") != "5" {
			t.Errorf("unexpected GDELT parameters: %s", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{
		  "articles": [{
		    "url": "https://example.com/world/story",
		    "title": "Sanctions announced",
		    "seendate": "20260929T120000Z",
		    "domain": "example.com",
		    "language": "English",
		    "sourcecountry": "United States"
		  }]
		}`)
	}))
	defer server.Close()

	fetcher, err := NewFetcher(server.Client(), "test-agent")
	if err != nil {
		t.Fatalf("new fetcher: %v", err)
	}
	fetcher.now = func() time.Time {
		return time.Date(2026, 9, 29, 12, 1, 0, 0, time.UTC)
	}
	articles, err := fetcher.Fetch(context.Background(), Source{
		ID:         "gdelt-test",
		Name:       "GDELT Test",
		Kind:       SourceKindGDELT,
		Tier:       "D",
		SourceType: "aggregator",
		URL:        server.URL + "/doc",
		Query:      "sanctions OR tariff",
		Language:   "en",
		MaxItems:   5,
	})
	if err != nil {
		t.Fatalf("fetch GDELT: %v", err)
	}
	if len(articles) != 1 {
		t.Fatalf("article count = %d, want 1", len(articles))
	}
	if articles[0].Publisher != "example.com" ||
		articles[0].Country != "United States" ||
		articles[0].Tier != "D" {
		t.Fatalf("unexpected article: %#v", articles[0])
	}
}
