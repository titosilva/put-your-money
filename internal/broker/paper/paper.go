// Package paper implements a simulated broker: it never sends a real order
// anywhere. It gets quotes from an underlying QuoteSource (a real broker's
// data feed, or a synthetic generator) and fills orders against those quotes
// with a simple slippage/fee model, tracking its own cash and positions.
//
// This is the adapter that makes the "never real money" guarantee: no
// matter which QuoteSource is plugged in, SubmitOrder here only ever
// mutates in-memory state.
package paper

import (
	"fmt"
	"math"
	"sync"

	"github.com/titosilva/put-your-money/internal/domain"
)

// QuoteSource supplies market data without exposing any execution
// capability. A broker.Adapter satisfies this automatically, since it's a
// superset — that's how PaperBroker can reuse Alpaca's data feed while
// keeping execution fully simulated.
type QuoteSource interface {
	GetMarketState(symbols []domain.Symbol) (domain.MarketState, error)
}

// Config controls the simulated execution model.
type Config struct {
	InitialCash float64
	SlippageBps float64 // basis points of adverse price movement applied on fill
	FeeBps      float64 // basis points of trade value charged as fee
}

func DefaultConfig(initialCash float64) Config {
	return Config{
		InitialCash: initialCash,
		SlippageBps: 5, // 0.05%
		FeeBps:      1, // 0.01%
	}
}

// Broker is a simulated execution venue holding its own portfolio. Create
// one instance per strategy run so runs never share state.
type Broker struct {
	name   string
	source QuoteSource
	cfg    Config

	mu        sync.Mutex
	portfolio domain.Portfolio
}

func New(name string, source QuoteSource, cfg Config) *Broker {
	return &Broker{
		name:   name,
		source: source,
		cfg:    cfg,
		portfolio: domain.Portfolio{
			Cash:      cfg.InitialCash,
			Positions: make(map[string]domain.Position),
		},
	}
}

func (b *Broker) Name() string { return b.name }

func (b *Broker) GetMarketState(symbols []domain.Symbol) (domain.MarketState, error) {
	return b.source.GetMarketState(symbols)
}

// Portfolio returns a snapshot of the current simulated holdings.
func (b *Broker) Portfolio() domain.Portfolio {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.copyPortfolio()
}

func (b *Broker) copyPortfolio() domain.Portfolio {
	positions := make(map[string]domain.Position, len(b.portfolio.Positions))
	for k, v := range b.portfolio.Positions {
		positions[k] = v
	}
	return domain.Portfolio{Cash: b.portfolio.Cash, Positions: positions}
}

// SubmitOrder simulates a fill against the given market state's quote for
// the order's symbol, applying slippage against the strategy and a fee.
// A Sell that exceeds (or starts from) a flat/long position opens or adds
// to a short position — needed for strategies like pairs trading that must
// short one leg. This is a simulator simplification: no margin requirement
// or borrow cost is modeled, so short size is only bounded by the fee
// itself ever exceeding available cash, not by any real broker's limits.
func (b *Broker) SubmitOrder(order domain.Order, state domain.MarketState) (domain.Fill, error) {
	quote, ok := state.Quote(order.Symbol.Ticker)
	if !ok {
		return domain.Fill{}, fmt.Errorf("paper broker: no quote for %s", order.Symbol.Ticker)
	}

	slippage := quote.Price * (b.cfg.SlippageBps / 10000)
	fillPrice := quote.Price
	switch order.Side {
	case domain.OrderSideBuy:
		fillPrice += slippage
	case domain.OrderSideSell:
		fillPrice -= slippage
	}

	notional := fillPrice * order.Qty
	fee := notional * (b.cfg.FeeBps / 10000)

	b.mu.Lock()
	defer b.mu.Unlock()

	if err := b.applyFill(order, fillPrice, fee); err != nil {
		return domain.Fill{}, err
	}

	return domain.Fill{
		Order:    order,
		Price:    fillPrice,
		Qty:      order.Qty,
		Fee:      fee,
		FilledAt: state.Timestamp,
	}, nil
}

func (b *Broker) applyFill(order domain.Order, fillPrice, fee float64) error {
	ticker := order.Symbol.Ticker
	pos := b.portfolio.Positions[ticker]
	pos.Symbol = order.Symbol

	notional := fillPrice * order.Qty

	var delta float64
	switch order.Side {
	case domain.OrderSideBuy:
		cost := notional + fee
		if cost > b.portfolio.Cash {
			return fmt.Errorf("paper broker: insufficient cash for %s: need %.2f, have %.2f", ticker, cost, b.portfolio.Cash)
		}
		b.portfolio.Cash -= cost
		delta = order.Qty

	case domain.OrderSideSell:
		b.portfolio.Cash += notional - fee
		delta = -order.Qty

	default:
		return fmt.Errorf("paper broker: unknown order side %q", order.Side)
	}

	newQty := pos.Qty + delta
	switch {
	case pos.Qty == 0:
		// opening a fresh position (long from Buy, short from Sell).
		pos.AvgPrice = fillPrice
	case sameSign(pos.Qty, newQty):
		if math.Abs(newQty) > math.Abs(pos.Qty) {
			// adding to the existing long or short exposure.
			added := math.Abs(newQty) - math.Abs(pos.Qty)
			pos.AvgPrice = (pos.AvgPrice*math.Abs(pos.Qty) + fillPrice*added) / math.Abs(newQty)
		}
		// otherwise partially reducing exposure: avg price is unchanged.
	default:
		// fully closed, or flipped from long to short (or vice versa): any
		// newly opened exposure starts fresh at this fill's price.
		pos.AvgPrice = fillPrice
	}
	pos.Qty = newQty

	if pos.Qty == 0 {
		delete(b.portfolio.Positions, ticker)
	} else {
		b.portfolio.Positions[ticker] = pos
	}
	return nil
}

func sameSign(a, b float64) bool {
	return (a > 0 && b > 0) || (a < 0 && b < 0)
}
