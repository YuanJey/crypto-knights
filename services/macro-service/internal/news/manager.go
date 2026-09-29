package news

import (
	"context"
	"sort"
	"sync"
	"time"
)

type Manager struct {
	store     *Store
	fetcher   *Fetcher
	sources   []Source
	mu        sync.RWMutex
	states    map[string]SourceState
	refreshMu sync.Mutex
	now       func() time.Time
}

func NewManager(store *Store, fetcher *Fetcher, sources []Source) *Manager {
	states := make(map[string]SourceState, len(sources))
	clonedSources := append([]Source(nil), sources...)
	for _, source := range clonedSources {
		status := "pending"
		if !source.Enabled {
			status = "disabled"
		}
		states[source.ID] = SourceState{
			Source: source,
			Status: status,
		}
	}
	return &Manager{
		store:   store,
		fetcher: fetcher,
		sources: clonedSources,
		states:  states,
		now:     time.Now,
	}
}

func (m *Manager) Start(ctx context.Context) {
	go func() {
		m.RefreshDue(ctx)
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				m.RefreshDue(ctx)
			}
		}
	}()
}

func (m *Manager) RefreshDue(ctx context.Context) []SourceState {
	m.refreshMu.Lock()
	defer m.refreshMu.Unlock()

	now := m.now().UTC()
	due := make([]Source, 0, len(m.sources))
	m.mu.Lock()
	for _, source := range m.sources {
		state := m.states[source.ID]
		if !source.Enabled {
			continue
		}
		if state.NextAttemptAt != nil && now.Before(*state.NextAttemptAt) {
			continue
		}
		attemptedAt := now
		nextAttemptAt := now.Add(source.PollInterval())
		state.Status = "refreshing"
		state.LastAttemptAt = &attemptedAt
		state.NextAttemptAt = &nextAttemptAt
		state.LastError = ""
		m.states[source.ID] = state
		due = append(due, source)
	}
	m.mu.Unlock()

	var waitGroup sync.WaitGroup
	for _, source := range due {
		source := source
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			articles, err := m.fetcher.Fetch(ctx, source)
			finishedAt := m.now().UTC()

			m.mu.Lock()
			state := m.states[source.ID]
			if err != nil {
				state.Status = "error"
				state.LastError = err.Error()
				state.LastItemCount = 0
			} else {
				state.Status = "healthy"
				state.LastSuccessAt = &finishedAt
				state.LastItemCount = len(articles)
				state.TotalItems += int64(m.store.Upsert(articles))
			}
			m.states[source.ID] = state
			m.mu.Unlock()
		}()
	}
	waitGroup.Wait()

	results := make([]SourceState, 0, len(due))
	m.mu.RLock()
	for _, source := range due {
		results = append(results, cloneSourceState(m.states[source.ID]))
	}
	m.mu.RUnlock()
	sort.Slice(results, func(i, j int) bool {
		return results[i].Source.ID < results[j].Source.ID
	})
	return results
}

func (m *Manager) Articles(filter ListFilter) []Article {
	return m.store.List(filter)
}

func (m *Manager) Sources() []SourceState {
	m.mu.RLock()
	states := make([]SourceState, 0, len(m.states))
	for _, state := range m.states {
		states = append(states, cloneSourceState(state))
	}
	m.mu.RUnlock()
	sort.Slice(states, func(i, j int) bool {
		return states[i].Source.ID < states[j].Source.ID
	})
	return states
}

func cloneSourceState(state SourceState) SourceState {
	if state.LastAttemptAt != nil {
		value := *state.LastAttemptAt
		state.LastAttemptAt = &value
	}
	if state.LastSuccessAt != nil {
		value := *state.LastSuccessAt
		state.LastSuccessAt = &value
	}
	if state.NextAttemptAt != nil {
		value := *state.NextAttemptAt
		state.NextAttemptAt = &value
	}
	return state
}
