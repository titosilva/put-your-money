package backtest_test

import (
	"testing"

	"github.com/titosilva/put-your-money/internal/backtest"
	"github.com/titosilva/put-your-money/internal/domain"
	"github.com/titosilva/put-your-money/internal/strategy"
	"github.com/titosilva/put-your-money/internal/strategy/bollinger"
	"github.com/titosilva/put-your-money/internal/strategy/macd"
	"github.com/titosilva/put-your-money/internal/strategy/movingaverage"
	"github.com/titosilva/put-your-money/internal/strategy/pairs"
	"github.com/titosilva/put-your-money/internal/strategy/rsi"
)

// TestBacktest_AllStrategiesRunCleanly is the automated benchmark: it
// replays every built-in strategy against the historical data in
// testdata/historical/ and checks each one completes without error and
// produces a sane result. It's not a claim that any strategy is
// profitable — see the numbers it logs for that — only a regression check
// that adding or changing a strategy hasn't broken the backtest path.
func TestBacktest_AllStrategiesRunCleanly(t *testing.T) {
	symbol := domain.Symbol{Ticker: "AAPL", Class: domain.AssetClassEquity}
	pairSymbol := domain.Symbol{Ticker: "MSFT", Class: domain.AssetClassEquity}

	source, err := backtest.NewHistoricalSource("../../testdata/historical", symbol, pairSymbol)
	if err != nil {
		t.Fatalf("loading historical data: %v", err)
	}
	if source.Len() == 0 {
		t.Fatal("historical source has no data")
	}

	const initialCash = 10_000
	strategies := []strategy.Strategy{
		movingaverage.New(symbol, 5, 10),
		rsi.New(symbol, 14, 30, 70, 10),
		macd.New(symbol, 12, 26, 9, 10),
		bollinger.New(symbol, 20, 2, 10),
		pairs.New(symbol, pairSymbol, 20, 2, 10),
	}

	for _, strat := range strategies {
		strat := strat
		t.Run(strat.Name(), func(t *testing.T) {
			result, err := backtest.Run(strat, source, initialCash)
			if err != nil {
				t.Fatalf("running backtest: %v", err)
			}
			if result.FinalEquity <= 0 {
				t.Fatalf("expected positive final equity, got %v", result.FinalEquity)
			}
			if len(result.EquityCurve) != source.Len() {
				t.Fatalf("expected %d equity snapshots, got %d", source.Len(), len(result.EquityCurve))
			}
			t.Logf("%s: final=$%.2f return=%.2f%% maxDrawdown=%.2f%% trades=%d",
				result.StrategyName, result.FinalEquity, result.TotalReturnPct, result.MaxDrawdownPct, result.NumTrades)
		})
	}
}
