package pairs

import (
	"testing"
	"time"

	"github.com/titosilva/put-your-money/internal/domain"
)

func tick(symA, symB domain.Symbol, priceA, priceB float64) domain.MarketState {
	return domain.MarketState{
		Timestamp: time.Now(),
		Quotes: map[string]domain.Quote{
			symA.Ticker: {Symbol: symA, Price: priceA},
			symB.Ticker: {Symbol: symB, Price: priceB},
		},
	}
}

func emptyPortfolio() domain.Portfolio {
	return domain.Portfolio{Positions: map[string]domain.Position{}}
}

// applyOrders is a tiny stand-in for the paper broker: it just tracks
// resulting position quantities so the test can drive multiple ticks
// without needing a full broker.
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

func TestPairsStrategy_DivergeAndConverge(t *testing.T) {
	symA := domain.Symbol{Ticker: "A"}
	symB := domain.Symbol{Ticker: "B"}
	strat := New(symA, symB, 5, 1.5, 10)

	portfolio := emptyPortfolio()

	// Seed + warm up the rolling window with a stable, non-diverging spread.
	for i := 0; i < 6; i++ {
		orders := strat.OnTick(tick(symA, symB, 100, 100), portfolio)
		if len(orders) != 0 {
			t.Fatalf("tick %d: expected no orders during warm-up, got %v", i, orders)
		}
	}

	// A shoots up relative to B: spread should exceed the upper band and
	// the strategy should short A, go long B.
	orders := strat.OnTick(tick(symA, symB, 140, 100), portfolio)
	if len(orders) != 2 {
		t.Fatalf("expected 2 orders on divergence, got %d: %v", len(orders), orders)
	}
	var sawShortA, sawLongB bool
	for _, o := range orders {
		if o.Symbol == symA && o.Side == domain.OrderSideSell {
			sawShortA = true
		}
		if o.Symbol == symB && o.Side == domain.OrderSideBuy {
			sawLongB = true
		}
	}
	if !sawShortA || !sawLongB {
		t.Fatalf("expected short A + long B, got %v", orders)
	}
	portfolio = applyOrders(portfolio, orders)

	// Flat spread again (back near the rolling mean): should close both legs.
	orders = strat.OnTick(tick(symA, symB, 100, 100), portfolio)
	if len(orders) != 2 {
		t.Fatalf("expected 2 closing orders on reversion, got %d: %v", len(orders), orders)
	}
	portfolio = applyOrders(portfolio, orders)
	if len(portfolio.Positions) != 0 {
		t.Fatalf("expected flat portfolio after reversion, got %+v", portfolio.Positions)
	}
}
