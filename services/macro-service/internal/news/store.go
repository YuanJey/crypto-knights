package news

import (
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"
)

type Store struct {
	mu        sync.RWMutex
	articles  map[string]Article
	maxItems  int
	retention time.Duration
	now       func() time.Time
}

func NewStore(maxItems int, retention time.Duration) *Store {
	return &Store{
		articles:  make(map[string]Article),
		maxItems:  maxItems,
		retention: retention,
		now:       time.Now,
	}
}

func (s *Store) Upsert(articles []Article) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.now().UTC()
	cutoff := now.Add(-s.retention)
	for id, article := range s.articles {
		if articleTime(article).Before(cutoff) {
			delete(s.articles, id)
		}
	}

	added := 0
	for _, article := range articles {
		article.Title = strings.TrimSpace(article.Title)
		article.URL = canonicalURL(article.URL)
		if article.Title == "" || article.URL == "" || articleTime(article).Before(cutoff) {
			continue
		}
		if article.ID == "" {
			article.ID = articleID(article.URL)
		}
		if existing, exists := s.articles[article.ID]; exists {
			if tierPriority(article.Tier) < tierPriority(existing.Tier) {
				s.articles[article.ID] = cloneArticle(article)
			}
			continue
		}
		s.articles[article.ID] = cloneArticle(article)
		added++
	}

	s.trimLocked()
	return added
}

func (s *Store) List(filter ListFilter) []Article {
	s.mu.RLock()
	articles := make([]Article, 0, len(s.articles))
	for _, article := range s.articles {
		if filter.SourceID != "" && article.SourceID != filter.SourceID {
			continue
		}
		if filter.Tier != "" && article.Tier != filter.Tier {
			continue
		}
		if filter.Since != nil && articleTime(article).Before(*filter.Since) {
			continue
		}
		articles = append(articles, cloneArticle(article))
	}
	s.mu.RUnlock()

	sort.Slice(articles, func(i, j int) bool {
		left := articleTime(articles[i])
		right := articleTime(articles[j])
		if left.Equal(right) {
			return articles[i].ID < articles[j].ID
		}
		return left.After(right)
	})
	if filter.Limit > 0 && len(articles) > filter.Limit {
		articles = articles[:filter.Limit]
	}
	return articles
}

func (s *Store) trimLocked() {
	if s.maxItems <= 0 || len(s.articles) <= s.maxItems {
		return
	}
	articles := make([]Article, 0, len(s.articles))
	for _, article := range s.articles {
		articles = append(articles, article)
	}
	sort.Slice(articles, func(i, j int) bool {
		return articleTime(articles[i]).Before(articleTime(articles[j]))
	})
	for _, article := range articles[:len(articles)-s.maxItems] {
		delete(s.articles, article.ID)
	}
}

func articleTime(article Article) time.Time {
	if article.PublishedAt != nil {
		return article.PublishedAt.UTC()
	}
	return article.RetrievedAt.UTC()
}

func articleID(articleURL string) string {
	sum := sha256.Sum256([]byte(articleURL))
	return "news_" + hex.EncodeToString(sum[:12])
}

func canonicalURL(raw string) string {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Host == "" {
		return ""
	}
	parsed.Scheme = strings.ToLower(parsed.Scheme)
	parsed.Host = strings.ToLower(parsed.Host)
	parsed.Fragment = ""
	query := parsed.Query()
	for key := range query {
		lower := strings.ToLower(key)
		if strings.HasPrefix(lower, "utm_") ||
			lower == "gclid" ||
			lower == "fbclid" {
			query.Del(key)
		}
	}
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

func tierPriority(tier string) int {
	switch tier {
	case "A":
		return 1
	case "B":
		return 2
	case "C":
		return 3
	default:
		return 4
	}
}

func cloneArticle(article Article) Article {
	if article.PublishedAt != nil {
		publishedAt := *article.PublishedAt
		article.PublishedAt = &publishedAt
	}
	return article
}
