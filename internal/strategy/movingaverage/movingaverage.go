// Package movingaverage implements a simple moving-average crossover
// strategy, included as a working example of the Strategy interface — not
// as a strategy anyone should expect to be profitable.
package movingaverage

import (
	"github.com/titosilva/put-your-money/internal/domain"
)

// Strategy buys when the price crosses above its moving average and sells
// when it crosses back below, using a fixed trade size.
type Strategy struct {
	symbol   domain.Symbol
	window   int
	tradeQty float64

	prices     []float64
	wasAbove   bool
	hasCrossed bool
}

func New(symbol domain.Symbol, window int, tradeQty float64) *Strategy {
	return &Strategy{
		symbol:   symbol,
		window:   window,
		tradeQty: tradeQty,
	}
}

func (s *Strategy) Name() string { return "moving-average-crossover" }

func (s *Strategy) Symbols() []domain.Symbol {
	return []domain.Symbol{s.symbol}
}

func (s *Strategy) OnTick(state domain.MarketState, portfolio domain.Portfolio) []domain.Order {
	quote, ok := state.Quote(s.symbol.Ticker)
	if !ok {
		return nil
	}

	s.prices = append(s.prices, quote.Price)
	if len(s.prices) > s.window {
		s.prices = s.prices[len(s.prices)-s.window:]
	}
	if len(s.prices) < s.window {
		return nil // not enough history yet
	}

	avg := average(s.prices)
	isAbove := quote.Price > avg

	defer func() { s.wasAbove = isAbove; s.hasCrossed = true }()
	if !s.hasCrossed {
		return nil // first reading with a full window; nothing to cross yet
	}
	if isAbove == s.wasAbove {
		return nil // no crossover
	}

	position := portfolio.Positions[s.symbol.Ticker]

	if isAbove && position.Qty == 0 {
		return []domain.Order{{Symbol: s.symbol, Side: domain.OrderSideBuy, Qty: s.tradeQty, CreatedAt: state.Timestamp}}
	}
	if !isAbove && position.Qty > 0 {
		return []domain.Order{{Symbol: s.symbol, Side: domain.OrderSideSell, Qty: position.Qty, CreatedAt: state.Timestamp}}
	}
	return nil
}

func average(xs []float64) float64 {
	var sum float64
	for _, x := range xs {
		sum += x
	}
	return sum / float64(len(xs))
}
