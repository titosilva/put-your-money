package pead

import (
	"testing"
	"time"

	"github.com/titosilva/put-your-money/internal/domain"
)

func tick(sym domain.Symbol, price float64, signal *domain.Signal) domain.MarketState {
	state := domain.MarketState{
		Timestamp: time.Now(),
		Quotes: map[string]domain.Quote{
			sym.Ticker: {Symbol: sym, Price: price},
		},
		Signals: map[string]domain.Signal{},
	}
	if signal != nil {
		state.Signals[signal.Name] = *signal
	}
	return state
}

func emptyPortfolio() domain.Portfolio {
	return domain.Portfolio{Positions: map[string]domain.Position{}}
}

func applyOrders(p domain.Portfolio, orders []domain.Order) domain.Portfolio {
	for _, o := range orders {
		pos := p.Positions[o.Symbol.Ticker]
		pos.Symbol = o.Symbol
		if o.Side == domain.OrderSideBuy {
			pos.Qty += o.Qty
		} else {
			pos.Qty -= o.Qty
		}
		if pos.Qty == 0 {
			delete(p.Positions, o.Symbol.Ticker)
		} else {
			p.Positions[o.Symbol.Ticker] = pos
		}
	}
	return p
}

func TestPEAD_PositiveSurpriseThenDrift(t *testing.T) {
	sym := domain.Symbol{Ticker: "AAPL"}
	strat := New(sym, 2.0, 3, 10) // enter at |surprise| >= 2%, hold 3 ticks

	portfolio := emptyPortfolio()

	// No signal: nothing happens.
	orders := strat.OnTick(tick(sym, 100, nil), portfolio)
	if len(orders) != 0 {
		t.Fatalf("expected no orders without a signal, got %v", orders)
	}

	// Big positive surprise: should go long.
	sig := &domain.Signal{Name: "AAPL:earnings_surprise_pct", Value: 5.0}
	orders = strat.OnTick(tick(sym, 100, sig), portfolio)
	if len(orders) != 1 || orders[0].Side != domain.OrderSideBuy {
		t.Fatalf("expected a single buy order, got %v", orders)
	}
	portfolio = applyOrders(portfolio, orders)

	// Holding: no signal ticks 1 and 2 should not close the position yet.
	for i := 0; i < 2; i++ {
		orders = strat.OnTick(tick(sym, 101, nil), portfolio)
		if len(orders) != 0 {
			t.Fatalf("tick %d: expected to still be holding, got orders %v", i, orders)
		}
	}

	// Drift window elapsed: should close (sell) the position.
	orders = strat.OnTick(tick(sym, 102, nil), portfolio)
	if len(orders) != 1 || orders[0].Side != domain.OrderSideSell {
		t.Fatalf("expected a single sell order closing the drift position, got %v", orders)
	}
	portfolio = applyOrders(portfolio, orders)
	if len(portfolio.Positions) != 0 {
		t.Fatalf("expected flat portfolio after drift window, got %+v", portfolio.Positions)
	}
}

func TestPEAD_NegativeSurpriseGoesShort(t *testing.T) {
	sym := domain.Symbol{Ticker: "AAPL"}
	strat := New(sym, 2.0, 3, 10)
	portfolio := emptyPortfolio()

	sig := &domain.Signal{Name: "AAPL:earnings_surprise_pct", Value: -6.0}
	orders := strat.OnTick(tick(sym, 100, sig), portfolio)
	if len(orders) != 1 || orders[0].Side != domain.OrderSideSell {
		t.Fatalf("expected a single sell (short) order, got %v", orders)
	}
}

func TestPEAD_SurpriseBelowThresholdIgnored(t *testing.T) {
	sym := domain.Symbol{Ticker: "AAPL"}
	strat := New(sym, 2.0, 3, 10)
	portfolio := emptyPortfolio()

	sig := &domain.Signal{Name: "AAPL:earnings_surprise_pct", Value: 0.5}
	orders := strat.OnTick(tick(sym, 100, sig), portfolio)
	if len(orders) != 0 {
		t.Fatalf("expected no orders for a below-threshold surprise, got %v", orders)
	}
}
