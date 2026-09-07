// Package memory is an in-memory storage.Store, good enough for a single
// day-long simulation run or for development. Swap for a Postgres/SQLite
// implementation of storage.Store later without touching engine or API code.
package memory

import (
	"fmt"
	"sync"

	"github.com/titosilva/put-your-money/internal/domain"
)

type Store struct {
	mu       sync.Mutex
	fills    map[string][]domain.Fill
	equity   map[string][]domain.EquitySnapshot
	runOrder []string
}

func New() *Store {
	return &Store{
		fills:  make(map[string][]domain.Fill),
		equity: make(map[string][]domain.EquitySnapshot),
	}
}

func (s *Store) SaveFill(runID string, fill domain.Fill) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.registerRun(runID)
	s.fills[runID] = append(s.fills[runID], fill)
	return nil
}

func (s *Store) SaveEquitySnapshot(snapshot domain.EquitySnapshot) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.registerRun(snapshot.RunID)
	s.equity[snapshot.RunID] = append(s.equity[snapshot.RunID], snapshot)
	return nil
}

func (s *Store) GetFills(runID string) ([]domain.Fill, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	fills, ok := s.fills[runID]
	if !ok {
		return nil, fmt.Errorf("memory store: unknown run %q", runID)
	}
	out := make([]domain.Fill, len(fills))
	copy(out, fills)
	return out, nil
}

func (s *Store) GetEquityCurve(runID string) ([]domain.EquitySnapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	curve, ok := s.equity[runID]
	if !ok {
		return nil, fmt.Errorf("memory store: unknown run %q", runID)
	}
	out := make([]domain.EquitySnapshot, len(curve))
	copy(out, curve)
	return out, nil
}

func (s *Store) ListRuns() ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, len(s.runOrder))
	copy(out, s.runOrder)
	return out, nil
}

// registerRun must be called with s.mu held.
func (s *Store) registerRun(runID string) {
	if _, ok := s.fills[runID]; ok {
		return
	}
	if _, ok := s.equity[runID]; ok {
		return
	}
	s.runOrder = append(s.runOrder, runID)
}
