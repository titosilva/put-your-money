// Package broker defines the adapter interface between the engine and any
// execution/data venue (a paper simulator, Alpaca, a Brazilian corretora,
// a crypto exchange testnet, ...). Strategies never see this interface —
// only the engine does — which is what lets a broker be swapped without
// touching a single strategy.
package broker

import "github.com/titosilva/put-your-money/internal/domain"

// Adapter is implemented once per venue. All methods operate in terms of
// domain types only, never anything venue-specific.
type Adapter interface {
	// Name identifies the venue for logging/dashboard display.
	Name() string

	// GetMarketState fetches the latest quotes for the given symbols.
	GetMarketState(symbols []domain.Symbol) (domain.MarketState, error)

	// SubmitOrder executes an order (simulated or real) and returns the Fill.
	SubmitOrder(order domain.Order, state domain.MarketState) (domain.Fill, error)
}
