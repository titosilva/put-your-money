// Package strategy defines the interface every trading strategy implements.
// The engine only ever talks to strategies through this interface — it has
// no idea whether a strategy trades one stock or ten different asset
// classes plus an external sentiment feed.
package strategy

import "github.com/titosilva/put-your-money/internal/domain"

// Strategy reacts to each new MarketState and current Portfolio by emitting
// zero or more Orders. It must not hold a reference to a broker or any I/O —
// all it knows is the data it's handed.
type Strategy interface {
	// Name identifies the strategy for logging/dashboard display.
	Name() string

	// Symbols declares which symbols this strategy wants MarketState quotes
	// for. The engine uses this to know what to fetch from the broker.
	Symbols() []domain.Symbol

	// OnTick is called once per engine tick with the latest market state and
	// the strategy's own current portfolio. It returns the orders to submit.
	OnTick(state domain.MarketState, portfolio domain.Portfolio) []domain.Order
}
