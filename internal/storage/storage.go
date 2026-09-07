// Package storage defines how a strategy run's history gets persisted, so
// the dashboard can show trades and equity curves after (or during) a run.
// The engine only depends on this interface — swapping the in-memory
// implementation for Postgres/SQLite later doesn't touch engine or API code.
package storage

import "github.com/titosilva/put-your-money/internal/domain"

// Store persists everything a single strategy run produces.
type Store interface {
	SaveFill(runID string, fill domain.Fill) error
	SaveEquitySnapshot(snapshot domain.EquitySnapshot) error

	GetFills(runID string) ([]domain.Fill, error)
	GetEquityCurve(runID string) ([]domain.EquitySnapshot, error)
	ListRuns() ([]string, error)
}
