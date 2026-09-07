package backtest

import (
	"github.com/titosilva/put-your-money/internal/broker/paper"
	"github.com/titosilva/put-your-money/internal/domain"
	"github.com/titosilva/put-your-money/internal/strategy"
)

// Result summarizes one strategy's performance over a full historical
// replay, without needing to inspect the raw equity curve.
type Result struct {
	StrategyName   string
	InitialCash    float64
	FinalEquity    float64
	TotalReturnPct float64
	MaxDrawdownPct float64
	NumTrades      int
	EquityCurve    []domain.EquitySnapshot
}

// Run replays every bar in source against strat, using an isolated
// PaperBroker seeded with initialCash. It resets source's cursor first, so
// multiple strategies can be benchmarked against the same loaded data.
//
// Unlike engine.Run, this has no ticker and no wall-clock wait: it steps
// through the entire dataset as fast as the strategy's OnTick runs, which
// is what makes it useful as a fast, repeatable benchmark.
func Run(strat strategy.Strategy, source *HistoricalSource, initialCash float64) (Result, error) {
	source.Reset()
	broker := paper.New(strat.Name(), source, paper.DefaultConfig(initialCash))

	var equityCurve []domain.EquitySnapshot
	numTrades := 0
	peakEquity := initialCash
	maxDrawdownPct := 0.0

	for !source.Done() {
		state, err := broker.GetMarketState(strat.Symbols())
		if err != nil {
			return Result{}, err
		}

		portfolio := broker.Portfolio()
		for _, order := range strat.OnTick(state, portfolio) {
			if _, err := broker.SubmitOrder(order, state); err != nil {
				continue // e.g. insufficient cash; skip like engine.Run does
			}
			numTrades++
		}

		portfolio = broker.Portfolio()
		equity := portfolio.Equity(state)
		if equity > peakEquity {
			peakEquity = equity
		}
		if drawdownPct := (peakEquity - equity) / peakEquity * 100; drawdownPct > maxDrawdownPct {
			maxDrawdownPct = drawdownPct
		}

		equityCurve = append(equityCurve, domain.EquitySnapshot{
			RunID:     strat.Name(),
			Timestamp: state.Timestamp,
			Equity:    equity,
			Cash:      portfolio.Cash,
		})
	}

	finalEquity := initialCash
	if len(equityCurve) > 0 {
		finalEquity = equityCurve[len(equityCurve)-1].Equity
	}

	return Result{
		StrategyName:   strat.Name(),
		InitialCash:    initialCash,
		FinalEquity:    finalEquity,
		TotalReturnPct: (finalEquity - initialCash) / initialCash * 100,
		MaxDrawdownPct: maxDrawdownPct,
		NumTrades:      numTrades,
		EquityCurve:    equityCurve,
	}, nil
}
