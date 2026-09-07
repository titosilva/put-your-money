// Package engine ties a Strategy to a broker.Adapter and a storage.Store,
// driving the tick loop and recording everything for the dashboard. This is
// the only place that knows about all three interfaces at once — strategies,
// adapters, and stores never reference each other directly.
package engine

import (
	"log"
	"time"

	"github.com/titosilva/put-your-money/internal/broker"
	"github.com/titosilva/put-your-money/internal/domain"
	"github.com/titosilva/put-your-money/internal/storage"
	"github.com/titosilva/put-your-money/internal/strategy"
)

// PortfolioReader is implemented by broker adapters (like paper.Broker) that
// track their own portfolio state. Adapters that only execute against a real
// account (like the Alpaca adapter) don't need to satisfy this — the engine
// falls back to tracking fills itself in that case.
type PortfolioReader interface {
	Portfolio() domain.Portfolio
}

// Run wires one Strategy to one broker.Adapter for a single simulation run.
type Run struct {
	ID       string
	Strategy strategy.Strategy
	Broker   broker.Adapter
	Store    storage.Store

	// TickInterval controls how often OnTick is called. A live day-long run
	// might use time.Minute; a fast local test can use a much shorter value.
	TickInterval time.Duration
}

// Start runs the tick loop until stop is closed. It blocks the calling
// goroutine — callers typically launch it with `go run.Start(stop)`.
func (r *Run) Start(stop <-chan struct{}) {
	ticker := time.NewTicker(r.TickInterval)
	defer ticker.Stop()

	log.Printf("[%s] run %q started on broker %q", r.Strategy.Name(), r.ID, r.Broker.Name())

	for {
		select {
		case <-stop:
			log.Printf("[%s] run %q stopped", r.Strategy.Name(), r.ID)
			return
		case <-ticker.C:
			r.tick()
		}
	}
}

func (r *Run) tick() {
	state, err := r.Broker.GetMarketState(r.Strategy.Symbols())
	if err != nil {
		log.Printf("[%s] run %q: fetching market state: %v", r.Strategy.Name(), r.ID, err)
		return
	}

	portfolio := r.currentPortfolio()

	orders := r.Strategy.OnTick(state, portfolio)
	for _, order := range orders {
		fill, err := r.Broker.SubmitOrder(order, state)
		if err != nil {
			log.Printf("[%s] run %q: order rejected: %v", r.Strategy.Name(), r.ID, err)
			continue
		}
		if err := r.Store.SaveFill(r.ID, fill); err != nil {
			log.Printf("[%s] run %q: saving fill: %v", r.Strategy.Name(), r.ID, err)
		}
	}

	// Re-read portfolio after fills so the equity snapshot reflects them.
	portfolio = r.currentPortfolio()
	snapshot := domain.EquitySnapshot{
		RunID:     r.ID,
		Timestamp: state.Timestamp,
		Equity:    portfolio.Equity(state),
		Cash:      portfolio.Cash,
	}
	if err := r.Store.SaveEquitySnapshot(snapshot); err != nil {
		log.Printf("[%s] run %q: saving equity snapshot: %v", r.Strategy.Name(), r.ID, err)
	}
}

func (r *Run) currentPortfolio() domain.Portfolio {
	if reader, ok := r.Broker.(PortfolioReader); ok {
		return reader.Portfolio()
	}
	return domain.Portfolio{Positions: map[string]domain.Position{}}
}
