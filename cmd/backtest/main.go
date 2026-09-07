// Command backtest runs every built-in strategy against the historical data
// stored under testdata/historical/ (see cmd/fetchdata) and prints a
// comparison table. It's the automated-benchmark counterpart to
// cmd/engine's live/synthetic runs: no wall-clock waiting, no network
// access, just replaying already-fetched bars — so a new strategy can be
// benchmarked in milliseconds, and CI can run this on every change.
package main

import (
	"fmt"
	"log"
	"os"
	"text/tabwriter"

	"github.com/titosilva/put-your-money/internal/backtest"
	"github.com/titosilva/put-your-money/internal/domain"
	"github.com/titosilva/put-your-money/internal/strategy"
	"github.com/titosilva/put-your-money/internal/strategy/bollinger"
	"github.com/titosilva/put-your-money/internal/strategy/macd"
	"github.com/titosilva/put-your-money/internal/strategy/movingaverage"
	"github.com/titosilva/put-your-money/internal/strategy/pairs"
	"github.com/titosilva/put-your-money/internal/strategy/rsi"
)

const initialCash = 10_000

func main() {
	symbol := domain.Symbol{Ticker: "AAPL", Class: domain.AssetClassEquity}
	pairSymbol := domain.Symbol{Ticker: "MSFT", Class: domain.AssetClassEquity}

	source, err := backtest.NewHistoricalSource("testdata/historical", symbol, pairSymbol)
	if err != nil {
		log.Fatalf("loading historical data: %v", err)
	}
	log.Printf("loaded %d aligned trading days for %s/%s", source.Len(), symbol.Ticker, pairSymbol.Ticker)

	strategies := []strategy.Strategy{
		movingaverage.New(symbol, 5, 10),
		rsi.New(symbol, 14, 30, 70, 10),
		macd.New(symbol, 12, 26, 9, 10),
		bollinger.New(symbol, 20, 2, 10),
		pairs.New(symbol, pairSymbol, 20, 2, 10),
	}

	results := make([]backtest.Result, 0, len(strategies))
	for _, strat := range strategies {
		result, err := backtest.Run(strat, source, initialCash)
		if err != nil {
			log.Fatalf("running %s: %v", strat.Name(), err)
		}
		results = append(results, result)
	}

	printTable(results)
}

func printTable(results []backtest.Result) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "STRATEGY\tFINAL EQUITY\tTOTAL RETURN\tMAX DRAWDOWN\tTRADES")
	for _, r := range results {
		fmt.Fprintf(w, "%s\t$%.2f\t%.2f%%\t%.2f%%\t%d\n",
			r.StrategyName, r.FinalEquity, r.TotalReturnPct, r.MaxDrawdownPct, r.NumTrades)
	}
	w.Flush()
}
