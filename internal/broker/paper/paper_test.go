package paper

import (
	"testing"
	"time"

	"github.com/titosilva/put-your-money/internal/domain"
)

type fixedSource struct{}

func (fixedSource) GetMarketState([]domain.Symbol) (domain.MarketState, error) {
	return domain.MarketState{}, nil // unused: tests call SubmitOrder with an explicit state
}

func stateAt(ticker string, price float64) domain.MarketState {
	return domain.MarketState{
		Timestamp: time.Now(),
		Quotes: map[string]domain.Quote{
			ticker: {Symbol: domain.Symbol{Ticker: ticker}, Price: price},
		},
	}
}

func noFeesConfig(cash float64) Config {
	return Config{InitialCash: cash, SlippageBps: 0, FeeBps: 0}
}

func TestSubmitOrder_OpenAndCoverShort(t *testing.T) {
	b := New("test", fixedSource{}, noFeesConfig(10_000))
	sym := domain.Symbol{Ticker: "AAPL"}

	// Sell without an existing position opens a short.
	fill, err := b.SubmitOrder(domain.Order{Symbol: sym, Side: domain.OrderSideSell, Qty: 10}, stateAt("AAPL", 100))
	if err != nil {
		t.Fatalf("opening short: %v", err)
	}
	if fill.Price != 100 {
		t.Fatalf("expected fill price 100, got %v", fill.Price)
	}
	pos := b.Portfolio().Positions["AAPL"]
	if pos.Qty != -10 {
		t.Fatalf("expected short position of -10, got %v", pos.Qty)
	}
	if b.Portfolio().Cash != 11_000 {
		t.Fatalf("expected cash 11000 after short proceeds, got %v", b.Portfolio().Cash)
	}

	// Price drops, we cover at a profit.
	_, err = b.SubmitOrder(domain.Order{Symbol: sym, Side: domain.OrderSideBuy, Qty: 10}, stateAt("AAPL", 80))
	if err != nil {
		t.Fatalf("covering short: %v", err)
	}
	pos = b.Portfolio().Positions["AAPL"]
	if pos.Qty != 0 {
		t.Fatalf("expected flat position after covering, got %v", pos.Qty)
	}
	wantCash := 11_000.0 - 800.0 // bought back 10 @ 80
	if b.Portfolio().Cash != wantCash {
		t.Fatalf("expected cash %v after covering, got %v", wantCash, b.Portfolio().Cash)
	}
}

func TestSubmitOrder_FlipLongToShort(t *testing.T) {
	b := New("test", fixedSource{}, noFeesConfig(10_000))
	sym := domain.Symbol{Ticker: "AAPL"}

	if _, err := b.SubmitOrder(domain.Order{Symbol: sym, Side: domain.OrderSideBuy, Qty: 5}, stateAt("AAPL", 100)); err != nil {
		t.Fatalf("opening long: %v", err)
	}
	if got := b.Portfolio().Positions["AAPL"].Qty; got != 5 {
		t.Fatalf("expected long 5, got %v", got)
	}

	// Sell more than the long position: should flip to a net short.
	if _, err := b.SubmitOrder(domain.Order{Symbol: sym, Side: domain.OrderSideSell, Qty: 8}, stateAt("AAPL", 110)); err != nil {
		t.Fatalf("flipping to short: %v", err)
	}
	pos := b.Portfolio().Positions["AAPL"]
	if pos.Qty != -3 {
		t.Fatalf("expected net short of -3 after flip, got %v", pos.Qty)
	}
	if pos.AvgPrice != 110 {
		t.Fatalf("expected new short leg avg price 110, got %v", pos.AvgPrice)
	}
}

func TestSubmitOrder_InsufficientCashForBuyStillRejected(t *testing.T) {
	b := New("test", fixedSource{}, noFeesConfig(100))
	sym := domain.Symbol{Ticker: "AAPL"}

	_, err := b.SubmitOrder(domain.Order{Symbol: sym, Side: domain.OrderSideBuy, Qty: 10}, stateAt("AAPL", 100))
	if err == nil {
		t.Fatal("expected insufficient cash error, got nil")
	}
}
