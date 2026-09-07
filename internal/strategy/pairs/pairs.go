// Package pairs implements the classical "distance method" pairs trading /
// statistical arbitrage strategy (Gatev, Goetzmann & Rouwenhorst, 2006):
// track the spread between two normalized price series, short the
// outperformer and long the underperformer when the spread diverges
// beyond a threshold, and close both legs when it reverts to its mean.
//
// This is the first strategy in this project that trades more than one
// symbol, and the first that needs short selling — both are handled purely
// through the existing Strategy/broker.Adapter interfaces: the engine and
// broker don't know or care that this strategy holds two legs at once.
package pairs

import (
	"math"
	"time"

	"github.com/titosilva/put-your-money/internal/domain"
	"github.com/titosilva/put-your-money/internal/indicators"
)

type Strategy struct {
	symbolA, symbolB domain.Symbol
	tradeQty         float64
	bands            *indicators.Bollinger

	firstPriceA, firstPriceB float64
	initialized              bool
}

// New creates a pairs trading strategy on symbolA/symbolB. window and
// entryK control the rolling spread statistics: a position opens once the
// normalized spread moves entryK standard deviations from its mean over the
// trailing window, and closes when the spread reverts back to that mean.
// Gatev et al.'s original entryK is 2.
func New(symbolA, symbolB domain.Symbol, window int, entryK, tradeQty float64) *Strategy {
	return &Strategy{
		symbolA:  symbolA,
		symbolB:  symbolB,
		tradeQty: tradeQty,
		bands:    indicators.NewBollinger(window, entryK),
	}
}

func (s *Strategy) Name() string { return "pairs-trading" }

func (s *Strategy) Symbols() []domain.Symbol {
	return []domain.Symbol{s.symbolA, s.symbolB}
}

func (s *Strategy) OnTick(state domain.MarketState, portfolio domain.Portfolio) []domain.Order {
	quoteA, okA := state.Quote(s.symbolA.Ticker)
	quoteB, okB := state.Quote(s.symbolB.Ticker)
	if !okA || !okB {
		return nil
	}

	if !s.initialized {
		s.firstPriceA, s.firstPriceB = quoteA.Price, quoteB.Price
		s.initialized = true
		return nil
	}

	// Normalize each series to start at 1.0 so the spread compares relative
	// performance, not raw price levels (a $500 stock vs. a $50 one).
	normA := quoteA.Price / s.firstPriceA
	normB := quoteB.Price / s.firstPriceB
	spread := normA - normB

	mid, upper, lower, ready := s.bands.Update(spread)
	if !ready {
		return nil
	}

	posA := portfolio.Positions[s.symbolA.Ticker]
	posB := portfolio.Positions[s.symbolB.Ticker]

	if posA.Qty == 0 && posB.Qty == 0 {
		switch {
		case spread > upper:
			// A has outperformed B beyond its usual range: bet on
			// convergence by shorting A and going long B.
			return []domain.Order{
				{Symbol: s.symbolA, Side: domain.OrderSideSell, Qty: s.tradeQty, CreatedAt: state.Timestamp},
				{Symbol: s.symbolB, Side: domain.OrderSideBuy, Qty: s.tradeQty, CreatedAt: state.Timestamp},
			}
		case spread < lower:
			return []domain.Order{
				{Symbol: s.symbolA, Side: domain.OrderSideBuy, Qty: s.tradeQty, CreatedAt: state.Timestamp},
				{Symbol: s.symbolB, Side: domain.OrderSideSell, Qty: s.tradeQty, CreatedAt: state.Timestamp},
			}
		}
		return nil
	}

	// Holding a pair position: close both legs once the spread reverts to
	// its rolling mean.
	revertedToMid := (posA.Qty > 0 && spread >= mid) || (posA.Qty < 0 && spread <= mid)
	if !revertedToMid {
		return nil
	}

	var orders []domain.Order
	if posA.Qty != 0 {
		orders = append(orders, closingOrder(s.symbolA, posA, state.Timestamp))
	}
	if posB.Qty != 0 {
		orders = append(orders, closingOrder(s.symbolB, posB, state.Timestamp))
	}
	return orders
}

func closingOrder(symbol domain.Symbol, pos domain.Position, ts time.Time) domain.Order {
	side := domain.OrderSideSell
	if pos.Qty < 0 {
		side = domain.OrderSideBuy
	}
	return domain.Order{Symbol: symbol, Side: side, Qty: math.Abs(pos.Qty), CreatedAt: ts}
}
