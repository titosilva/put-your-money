// Command engine runs one or more strategy Runs against a broker adapter and
// serves the dashboard API + static frontend.
//
// By default it uses the synthetic random-walk QuoteSource under a
// PaperBroker, so it runs with zero configuration. In that mode it launches
// every built-in strategy at once, each with its own isolated portfolio but
// fed identical prices, so the dashboard can compare them side by side.
//
// Set APCA_API_KEY_ID and APCA_API_SECRET_KEY (Alpaca paper trading keys) to
// instead run a single strategy against Alpaca's live paper-trading market
// data and execution; pick it with STRATEGY=ma|rsi|macd|bollinger|pairs
// (default ma).
package main

import (
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/titosilva/put-your-money/internal/api"
	"github.com/titosilva/put-your-money/internal/broker"
	"github.com/titosilva/put-your-money/internal/broker/alpaca"
	"github.com/titosilva/put-your-money/internal/broker/paper"
	"github.com/titosilva/put-your-money/internal/broker/synthetic"
	"github.com/titosilva/put-your-money/internal/domain"
	"github.com/titosilva/put-your-money/internal/engine"
	"github.com/titosilva/put-your-money/internal/storage"
	"github.com/titosilva/put-your-money/internal/storage/memory"
	"github.com/titosilva/put-your-money/internal/strategy"
	"github.com/titosilva/put-your-money/internal/strategy/bollinger"
	"github.com/titosilva/put-your-money/internal/strategy/macd"
	"github.com/titosilva/put-your-money/internal/strategy/movingaverage"
	"github.com/titosilva/put-your-money/internal/strategy/pairs"
	"github.com/titosilva/put-your-money/internal/strategy/rsi"
)

func main() {
	symbol := domain.Symbol{Ticker: "AAPL", Class: domain.AssetClassEquity}
	pairSymbol := domain.Symbol{Ticker: "MSFT", Class: domain.AssetClassEquity}
	store := memory.New()

	runs := buildRuns(symbol, pairSymbol, store)

	stop := make(chan struct{})
	for _, r := range runs {
		go r.Start(stop)
	}

	server := api.New(store)
	mux := http.NewServeMux()
	mux.Handle("/api/", server)
	mux.Handle("/", http.FileServer(http.Dir("web")))

	addr := ":8080"
	httpServer := &http.Server{Addr: addr, Handler: mux}
	go func() {
		log.Printf("dashboard listening on http://localhost%s", addr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("http server: %v", err)
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	close(stop)
	log.Println("shutting down")
}

// buildRuns wires up the strategy Run(s) for this process. With Alpaca
// credentials configured it runs one strategy against the real paper
// account; otherwise it runs every built-in strategy concurrently against a
// shared synthetic feed, each with its own isolated PaperBroker so they can
// be compared fairly on identical prices.
func buildRuns(symbol, pairSymbol domain.Symbol, store storage.Store) []*engine.Run {
	keyID := os.Getenv("APCA_API_KEY_ID")
	secret := os.Getenv("APCA_API_SECRET_KEY")
	if keyID != "" && secret != "" {
		log.Println("using Alpaca paper trading adapter")
		adapter := alpaca.NewPaperAdapter(keyID, secret)
		strat := selectStrategy(os.Getenv("STRATEGY"), symbol, pairSymbol)
		return []*engine.Run{{
			ID:           "alpaca-" + strat.Name(),
			Strategy:     strat,
			Broker:       adapter,
			Store:        store,
			TickInterval: 5 * time.Second,
		}}
	}

	log.Println("no Alpaca credentials found, running all built-in strategies against synthetic data")
	source := synthetic.New(time.Now().UnixNano())
	source.SetInitialPrice(symbol.Ticker, 150)
	// pairSymbol tracks symbol's price plus a mean-reverting spread, so the
	// pairs-trading strategy below actually has something to converge on —
	// two independent random walks essentially never do.
	source.SetCompanion(pairSymbol.Ticker, symbol.Ticker, 1.8, 0.1, 0.01)

	strategies := []strategy.Strategy{
		movingaverage.New(symbol, 5, 10),
		rsi.New(symbol, 14, 30, 70, 10),
		macd.New(symbol, 12, 26, 9, 10),
		bollinger.New(symbol, 20, 2, 10),
		pairs.New(symbol, pairSymbol, 20, 2, 10),
	}

	runs := make([]*engine.Run, len(strategies))
	for i, strat := range strategies {
		adapter := newIsolatedPaperBroker(strat.Name(), source)
		runs[i] = &engine.Run{
			ID:           strat.Name(),
			Strategy:     strat,
			Broker:       adapter,
			Store:        store,
			TickInterval: 5 * time.Second,
		}
	}
	return runs
}

func newIsolatedPaperBroker(name string, source paper.QuoteSource) broker.Adapter {
	return paper.New(name, source, paper.DefaultConfig(10_000))
}

func selectStrategy(name string, symbol, pairSymbol domain.Symbol) strategy.Strategy {
	switch name {
	case "rsi":
		return rsi.New(symbol, 14, 30, 70, 10)
	case "macd":
		return macd.New(symbol, 12, 26, 9, 10)
	case "bollinger":
		return bollinger.New(symbol, 20, 2, 10)
	case "pairs":
		return pairs.New(symbol, pairSymbol, 20, 2, 10)
	default:
		return movingaverage.New(symbol, 5, 10)
	}
}
