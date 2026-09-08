// Package pead implements a simplified post-earnings-announcement drift
// strategy (Ball & Brown, 1968; see LITERATURE.md): on a large earnings
// surprise, trade in the direction of the surprise and hold for a fixed
// drift window, betting that the market underreacts to the news and the
// price keeps drifting the same way for some time afterward.
//
// This is the first strategy here driven by external, non-price input
// (an earnings-surprise event) rather than price action alone. It reads
// that input the way the architecture was designed for: as a
// domain.Signal in MarketState, populated here by
// backtest.HistoricalSource.AttachSignal from stored earnings-surprise
// data (cmd/fetchearnings) — nothing about Strategy, engine, or
// broker.Adapter needed to change to support it.
//
// Simplified vs. the academic construction in two ways, both because of
// what's practical with a single-symbol, free-data setup rather than a
// broad decile-ranked universe: (1) this trades one symbol at a time
// instead of ranking many stocks into long/short deciles, and (2) the
// signal itself is approximate — see cmd/fetchearnings and LITERATURE.md
// for why (Finnhub's free tier gives quarter-end dates, not actual
// announcement dates, and only the last 4 quarters of history).
package pead

import (
	"math"

	"github.com/titosilva/put-your-money/internal/domain"
)

type Strategy struct {
	symbol            domain.Symbol
	signalName        string
	entryThresholdPct float64
	driftTicks        int
	tradeQty          float64

	ticksInPosition int
}

// New creates a PEAD strategy for symbol. entryThresholdPct is the minimum
// |earnings surprise %| to act on (Ball & Brown-style studies typically
// use the most extreme decile of surprises; a few percent is a reasonable
// single-symbol stand-in). driftTicks is how many ticks to hold the
// resulting position before closing it regardless of new signals.
func New(symbol domain.Symbol, entryThresholdPct float64, driftTicks int, tradeQty float64) *Strategy {
	return &Strategy{
		symbol:            symbol,
		signalName:        symbol.Ticker + ":earnings_surprise_pct",
		entryThresholdPct: entryThresholdPct,
		driftTicks:        driftTicks,
		tradeQty:          tradeQty,
	}
}

func (s *Strategy) Name() string { return "pead-earnings-drift-" + s.symbol.Ticker }

func (s *Strategy) Symbols() []domain.Symbol {
	return []domain.Symbol{s.symbol}
}

func (s *Strategy) OnTick(state domain.MarketState, portfolio domain.Portfolio) []domain.Order {
	position := portfolio.Positions[s.symbol.Ticker]

	if position.Qty != 0 {
		s.ticksInPosition++
		if s.ticksInPosition < s.driftTicks {
			return nil
		}
		s.ticksInPosition = 0
		side := domain.OrderSideSell
		if position.Qty < 0 {
			side = domain.OrderSideBuy
		}
		return []domain.Order{{Symbol: s.symbol, Side: side, Qty: math.Abs(position.Qty), CreatedAt: state.Timestamp}}
	}

	signal, ok := state.Signals[s.signalName]
	if !ok {
		return nil
	}

	switch {
	case signal.Value >= s.entryThresholdPct:
		s.ticksInPosition = 0
		return []domain.Order{{Symbol: s.symbol, Side: domain.OrderSideBuy, Qty: s.tradeQty, CreatedAt: state.Timestamp}}
	case signal.Value <= -s.entryThresholdPct:
		s.ticksInPosition = 0
		return []domain.Order{{Symbol: s.symbol, Side: domain.OrderSideSell, Qty: s.tradeQty, CreatedAt: state.Timestamp}}
	}
	return nil
}
