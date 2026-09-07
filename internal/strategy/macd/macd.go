// Package macd implements a classical MACD crossover strategy (Appel):
// buy when the MACD line crosses above its signal line (bullish momentum
// shift), sell when it crosses back below. Price-only, single asset, no
// external input.
package macd

import (
	"github.com/titosilva/put-your-money/internal/domain"
	"github.com/titosilva/put-your-money/internal/indicators"
)

type Strategy struct {
	symbol   domain.Symbol
	macd     *indicators.MACD
	tradeQty float64

	prevMACD, prevSignal float64
	hasPrev              bool
}

// New creates a MACD strategy. Appel's original defaults are fast=12,
// slow=26, signal=9 (in whatever bar interval the engine ticks at).
func New(symbol domain.Symbol, fast, slow, signal int, tradeQty float64) *Strategy {
	return &Strategy{
		symbol:   symbol,
		macd:     indicators.NewMACD(fast, slow, signal),
		tradeQty: tradeQty,
	}
}

func (s *Strategy) Name() string { return "macd-crossover" }

func (s *Strategy) Symbols() []domain.Symbol {
	return []domain.Symbol{s.symbol}
}

func (s *Strategy) OnTick(state domain.MarketState, portfolio domain.Portfolio) []domain.Order {
	quote, ok := state.Quote(s.symbol.Ticker)
	if !ok {
		return nil
	}

	macdVal, signalVal, ready := s.macd.Update(quote.Price)
	if !ready {
		return nil
	}

	hadPrev := s.hasPrev
	prevMACD, prevSignal := s.prevMACD, s.prevSignal
	s.prevMACD, s.prevSignal, s.hasPrev = macdVal, signalVal, true

	if !hadPrev {
		return nil // first ready reading; nothing to cross yet
	}

	crossedUp := prevMACD <= prevSignal && macdVal > signalVal
	crossedDown := prevMACD >= prevSignal && macdVal < signalVal

	position := portfolio.Positions[s.symbol.Ticker]

	if crossedUp && position.Qty == 0 {
		return []domain.Order{{Symbol: s.symbol, Side: domain.OrderSideBuy, Qty: s.tradeQty, CreatedAt: state.Timestamp}}
	}
	if crossedDown && position.Qty > 0 {
		return []domain.Order{{Symbol: s.symbol, Side: domain.OrderSideSell, Qty: position.Qty, CreatedAt: state.Timestamp}}
	}
	return nil
}
