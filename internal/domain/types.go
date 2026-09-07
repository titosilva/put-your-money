// Package domain holds the core types shared by strategies, broker adapters,
// the engine, and storage. Nothing in here knows about any specific broker
// or data provider — that isolation is the whole point.
package domain

import "time"

// AssetClass distinguishes the kind of instrument a Symbol represents, so
// strategies and adapters can reason about assets beyond plain equities.
type AssetClass string

const (
	AssetClassEquity AssetClass = "equity"
	AssetClassCrypto AssetClass = "crypto"
	AssetClassForex  AssetClass = "forex"
)

// Symbol identifies a tradable instrument on a given venue.
type Symbol struct {
	Ticker string
	Class  AssetClass
}

func (s Symbol) String() string {
	return s.Ticker
}

// Quote is the latest known price for a Symbol.
type Quote struct {
	Symbol    Symbol
	Price     float64
	Timestamp time.Time
}

// Signal is a timestamped external input (news sentiment, an economic
// indicator, anything that isn't a price) that a strategy can subscribe to
// alongside price data via the same MarketState.
type Signal struct {
	Name      string
	Value     float64
	Timestamp time.Time
}

// MarketState is the snapshot handed to a strategy on every tick: the latest
// quotes for the symbols it's subscribed to, plus any external signals.
type MarketState struct {
	Timestamp time.Time
	Quotes    map[string]Quote  // keyed by Symbol.Ticker
	Signals   map[string]Signal // keyed by Signal.Name
}

func (m MarketState) Quote(ticker string) (Quote, bool) {
	q, ok := m.Quotes[ticker]
	return q, ok
}

// OrderSide is the direction of an order.
type OrderSide string

const (
	OrderSideBuy  OrderSide = "buy"
	OrderSideSell OrderSide = "sell"
)

// Order is what a Strategy emits; it says nothing about how it gets filled —
// that's the BrokerAdapter's job.
type Order struct {
	Symbol    Symbol
	Side      OrderSide
	Qty       float64
	CreatedAt time.Time
}

// Fill is what a BrokerAdapter returns once an Order has been executed
// (simulated or real).
type Fill struct {
	Order    Order
	Price    float64
	Qty      float64
	Fee      float64
	FilledAt time.Time
}

// Position is the current holding in a single symbol.
type Position struct {
	Symbol   Symbol
	Qty      float64
	AvgPrice float64
}

// Portfolio is the strategy-visible state of cash + holdings. Each strategy
// run gets its own Portfolio so multiple strategies can be compared fairly
// on identical market data without interfering with each other.
type Portfolio struct {
	Cash      float64
	Positions map[string]Position // keyed by Symbol.Ticker
}

// Equity returns cash + mark-to-market value of all positions given the
// latest quotes.
func (p Portfolio) Equity(state MarketState) float64 {
	total := p.Cash
	for ticker, pos := range p.Positions {
		if q, ok := state.Quote(ticker); ok {
			total += pos.Qty * q.Price
		} else {
			total += pos.Qty * pos.AvgPrice
		}
	}
	return total
}

// EquitySnapshot is a point-in-time record of a strategy run's performance,
// persisted so the dashboard can plot equity curves and compare runs.
type EquitySnapshot struct {
	RunID     string
	Timestamp time.Time
	Equity    float64
	Cash      float64
}
