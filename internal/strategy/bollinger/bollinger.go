// Package bollinger implements a classical Bollinger Bands mean-reversion
// strategy (Bollinger): buy when price drops below the lower band
// (statistically stretched relative to recent volatility), sell when it
// reverts back to the middle band. Price-only, single asset, no external
// input.
package bollinger

import (
	"github.com/titosilva/put-your-money/internal/domain"
	"github.com/titosilva/put-your-money/internal/indicators"
)

type Strategy struct {
	symbol   domain.Symbol
	bands    *indicators.Bollinger
	tradeQty float64
}

// New creates a Bollinger Bands strategy. Bollinger's original defaults are
// period=20, k=2 (standard deviations).
func New(symbol domain.Symbol, period int, k, tradeQty float64) *Strategy {
	return &Strategy{
		symbol:   symbol,
		bands:    indicators.NewBollinger(period, k),
		tradeQty: tradeQty,
	}
}

func (s *Strategy) Name() string { return "bollinger-mean-reversion" }

func (s *Strategy) Symbols() []domain.Symbol {
	return []domain.Symbol{s.symbol}
}

func (s *Strategy) OnTick(state domain.MarketState, portfolio domain.Portfolio) []domain.Order {
	quote, ok := state.Quote(s.symbol.Ticker)
	if !ok {
		return nil
	}

	mid, _, lower, ready := s.bands.Update(quote.Price)
	if !ready {
		return nil
	}

	position := portfolio.Positions[s.symbol.Ticker]

	if quote.Price <= lower && position.Qty == 0 {
		return []domain.Order{{Symbol: s.symbol, Side: domain.OrderSideBuy, Qty: s.tradeQty, CreatedAt: state.Timestamp}}
	}
	if quote.Price >= mid && position.Qty > 0 {
		return []domain.Order{{Symbol: s.symbol, Side: domain.OrderSideSell, Qty: position.Qty, CreatedAt: state.Timestamp}}
	}
	return nil
}
