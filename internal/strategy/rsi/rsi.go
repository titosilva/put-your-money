// Package rsi implements a classical RSI mean-reversion strategy (Wilder,
// 1978): buy when the Relative Strength Index signals oversold, sell when
// it signals overbought. Price-only, single asset, no external input.
package rsi

import (
	"github.com/titosilva/put-your-money/internal/domain"
	"github.com/titosilva/put-your-money/internal/indicators"
)

type Strategy struct {
	symbol     domain.Symbol
	rsi        *indicators.RSI
	oversold   float64
	overbought float64
	tradeQty   float64
}

// New creates an RSI strategy. Wilder's original defaults are period=14,
// oversold=30, overbought=70.
func New(symbol domain.Symbol, period int, oversold, overbought, tradeQty float64) *Strategy {
	return &Strategy{
		symbol:     symbol,
		rsi:        indicators.NewRSI(period),
		oversold:   oversold,
		overbought: overbought,
		tradeQty:   tradeQty,
	}
}

func (s *Strategy) Name() string { return "rsi-mean-reversion" }

func (s *Strategy) Symbols() []domain.Symbol {
	return []domain.Symbol{s.symbol}
}

func (s *Strategy) OnTick(state domain.MarketState, portfolio domain.Portfolio) []domain.Order {
	quote, ok := state.Quote(s.symbol.Ticker)
	if !ok {
		return nil
	}

	value, ready := s.rsi.Update(quote.Price)
	if !ready {
		return nil
	}

	position := portfolio.Positions[s.symbol.Ticker]

	if value < s.oversold && position.Qty == 0 {
		return []domain.Order{{Symbol: s.symbol, Side: domain.OrderSideBuy, Qty: s.tradeQty, CreatedAt: state.Timestamp}}
	}
	if value > s.overbought && position.Qty > 0 {
		return []domain.Order{{Symbol: s.symbol, Side: domain.OrderSideSell, Qty: position.Qty, CreatedAt: state.Timestamp}}
	}
	return nil
}
