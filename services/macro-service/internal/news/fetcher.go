package news

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const maxFeedBytes int64 = 8 << 20

type Fetcher struct {
	client    *http.Client
	userAgent string
	now       func() time.Time
}

func NewFetcher(client *http.Client, userAgent string) (*Fetcher, error) {
	if client == nil {
		return nil, errors.New("HTTP client is required")
	}
	if strings.TrimSpace(userAgent) == "" {
		return nil, errors.New("news user agent is required")
	}
	return &Fetcher{
		client:    client,
		userAgent: strings.TrimSpace(userAgent),
		now:       time.Now,
	}, nil
}

func (f *Fetcher) Fetch(ctx context.Context, source Source) ([]Article, error) {
	switch source.Kind {
	case SourceKindRSS:
		return f.fetchRSS(ctx, source)
	case SourceKindGDELT:
		return f.fetchGDELT(ctx, source)
	default:
		return nil, fmt.Errorf("unsupported source kind %q", source.Kind)
	}
}

func (f *Fetcher) fetchRSS(ctx context.Context, source Source) ([]Article, error) {
	document, responseURL, err := f.fetch(ctx, source.URL, "application/rss+xml, application/atom+xml, application/xml, text/xml")
	if err != nil {
		return nil, err
	}
	document = bytes.TrimPrefix(document, []byte{0xef, 0xbb, 0xbf})

	var feed xmlFeed
	decoder := xml.NewDecoder(bytes.NewReader(document))
	decoder.CharsetReader = supportedCharsetReader
	if err := decoder.Decode(&feed); err != nil {
		return nil, fmt.Errorf("decode RSS/Atom from %s: %w", source.ID, err)
	}
	retrievedAt := f.now().UTC()
	articles := make([]Article, 0, len(feed.Channel.Items)+len(feed.Entries))
	for _, item := range feed.Channel.Items {
		link := resolveURL(responseURL, item.Link)
		publisher := strings.TrimSpace(item.Source.Name)
		if publisher == "" {
			publisher = source.Name
		}
		articles = append(articles, Article{
			SourceID:    source.ID,
			SourceName:  source.Name,
			Publisher:   publisher,
			Tier:        source.Tier,
			SourceType:  source.SourceType,
			Title:       strings.TrimSpace(item.Title),
			URL:         link,
			Language:    source.Language,
			Country:     source.Country,
			PublishedAt: parsePublishedAt(firstNonEmpty(item.PubDate, item.Date)),
			RetrievedAt: retrievedAt,
		})
	}
	for _, entry := range feed.Entries {
		articles = append(articles, Article{
			SourceID:    source.ID,
			SourceName:  source.Name,
			Publisher:   source.Name,
			Tier:        source.Tier,
			SourceType:  source.SourceType,
			Title:       strings.TrimSpace(entry.Title),
			URL:         selectAtomLink(responseURL, entry.Links),
			Language:    source.Language,
			Country:     source.Country,
			PublishedAt: parsePublishedAt(firstNonEmpty(entry.Published, entry.Updated)),
			RetrievedAt: retrievedAt,
		})
	}
	articles = validArticles(articles)
	if len(articles) > source.MaxItems {
		articles = articles[:source.MaxItems]
	}
	return articles, nil
}

func (f *Fetcher) fetchGDELT(ctx context.Context, source Source) ([]Article, error) {
	endpoint, err := url.Parse(source.URL)
	if err != nil {
		return nil, fmt.Errorf("parse GDELT endpoint: %w", err)
	}
	query := endpoint.Query()
	query.Set("query", source.Query)
	query.Set("mode", "artlist")
	query.Set("format", "json")
	query.Set("sort", "datedesc")
	query.Set("maxrecords", strconv.Itoa(source.MaxItems))
	endpoint.RawQuery = query.Encode()

	document, _, err := f.fetch(ctx, endpoint.String(), "application/json")
	if err != nil {
		return nil, err
	}
	var response gdeltResponse
	if err := json.Unmarshal(document, &response); err != nil {
		return nil, fmt.Errorf("decode GDELT response: %w", err)
	}
	retrievedAt := f.now().UTC()
	articles := make([]Article, 0, len(response.Articles))
	for _, item := range response.Articles {
		articles = append(articles, Article{
			SourceID:    source.ID,
			SourceName:  source.Name,
			Publisher:   firstNonEmpty(strings.TrimSpace(item.Domain), source.Name),
			Tier:        source.Tier,
			SourceType:  source.SourceType,
			Title:       strings.TrimSpace(item.Title),
			URL:         strings.TrimSpace(item.URL),
			Language:    strings.ToLower(firstNonEmpty(item.Language, source.Language)),
			Country:     firstNonEmpty(item.SourceCountry, source.Country),
			PublishedAt: parsePublishedAt(item.SeenDate),
			RetrievedAt: retrievedAt,
		})
	}
	return validArticles(articles), nil
}

func (f *Fetcher) fetch(
	ctx context.Context,
	endpoint string,
	accept string,
) ([]byte, *url.URL, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("create feed request: %w", err)
	}
	request.Header.Set("Accept", accept)
	request.Header.Set("User-Agent", f.userAgent)

	response, err := f.client.Do(request)
	if err != nil {
		return nil, nil, fmt.Errorf("fetch %s: %w", endpoint, err)
	}
	defer response.Body.Close()
	document, err := io.ReadAll(io.LimitReader(response.Body, maxFeedBytes+1))
	if err != nil {
		return nil, nil, fmt.Errorf("read %s: %w", endpoint, err)
	}
	if int64(len(document)) > maxFeedBytes {
		return nil, nil, fmt.Errorf("feed %s exceeds %d bytes", endpoint, maxFeedBytes)
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		message := strings.TrimSpace(string(document))
		if len(message) > 300 {
			message = message[:300]
		}
		return nil, nil, fmt.Errorf(
			"fetch %s returned %d: %s",
			endpoint,
			response.StatusCode,
			message,
		)
	}
	return document, response.Request.URL, nil
}

type xmlFeed struct {
	Channel struct {
		Items []rssItem `xml:"item"`
	} `xml:"channel"`
	Entries []atomEntry `xml:"entry"`
}

type rssItem struct {
	Title   string `xml:"title"`
	Link    string `xml:"link"`
	PubDate string `xml:"pubDate"`
	Date    string `xml:"date"`
	Source  struct {
		Name string `xml:",chardata"`
		URL  string `xml:"url,attr"`
	} `xml:"source"`
}

type atomEntry struct {
	Title     string         `xml:"title"`
	Links     []atomFeedLink `xml:"link"`
	Published string         `xml:"published"`
	Updated   string         `xml:"updated"`
}

type atomFeedLink struct {
	Href string `xml:"href,attr"`
	Rel  string `xml:"rel,attr"`
}

type gdeltResponse struct {
	Articles []struct {
		URL           string `json:"url"`
		Title         string `json:"title"`
		SeenDate      string `json:"seendate"`
		Domain        string `json:"domain"`
		Language      string `json:"language"`
		SourceCountry string `json:"sourcecountry"`
	} `json:"articles"`
}

func selectAtomLink(base *url.URL, links []atomFeedLink) string {
	for _, link := range links {
		if link.Rel == "" || link.Rel == "alternate" {
			return resolveURL(base, link.Href)
		}
	}
	return ""
}

func resolveURL(base *url.URL, raw string) string {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return ""
	}
	return base.ResolveReference(parsed).String()
}

func parsePublishedAt(raw string) *time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	raw = normalizeZoneOffset(raw)
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		time.RFC1123Z,
		time.RFC1123,
		time.RFC822Z,
		time.RFC822,
		"Mon, 2 Jan 2006 15:04:05 -0700",
		"20060102T150405Z",
	}
	for _, layout := range layouts {
		parsed, err := time.Parse(layout, raw)
		if err == nil {
			utc := parsed.UTC()
			return &utc
		}
	}
	return nil
}

func normalizeZoneOffset(value string) string {
	offsets := map[string]string{
		" GMT": " +0000",
		" UTC": " +0000",
		" EST": " -0500",
		" EDT": " -0400",
		" CST": " -0600",
		" CDT": " -0500",
		" MST": " -0700",
		" MDT": " -0600",
		" PST": " -0800",
		" PDT": " -0700",
	}
	for suffix, offset := range offsets {
		if prefix, found := strings.CutSuffix(value, suffix); found {
			return prefix + offset
		}
	}
	return value
}

func supportedCharsetReader(label string, input io.Reader) (io.Reader, error) {
	switch strings.ToLower(strings.TrimSpace(label)) {
	case "utf-8", "utf8", "us-ascii", "ascii":
		return input, nil
	default:
		return nil, fmt.Errorf("unsupported XML encoding %q", label)
	}
}

func validArticles(articles []Article) []Article {
	valid := articles[:0]
	for _, article := range articles {
		if strings.TrimSpace(article.Title) == "" || canonicalURL(article.URL) == "" {
			continue
		}
		valid = append(valid, article)
	}
	return valid
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
