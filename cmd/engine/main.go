// Command engine runs one or more strategy Runs against a broker adapter and
// serves the dashboard API + static frontend.
//
// By default it uses the synthetic random-walk QuoteSource under a
// PaperBroker, so it runs with zero configuration. Set APCA_API_KEY_ID and
// APCA_API_SECRET_KEY (Alpaca paper trading keys) to instead run against
// Alpaca's live paper-trading market data and execution.
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
	"github.com/titosilva/put-your-money/internal/storage/memory"
	"github.com/titosilva/put-your-money/internal/strategy/movingaverage"
)

func main() {
	symbol := domain.Symbol{Ticker: "AAPL", Class: domain.AssetClassEquity}

	adapter := buildBrokerAdapter(symbol)
	store := memory.New()

	run := &engine.Run{
		ID:           "demo-ma-crossover",
		Strategy:     movingaverage.New(symbol, 5, 10),
		Broker:       adapter,
		Store:        store,
		TickInterval: 5 * time.Second,
	}

	stop := make(chan struct{})
	go run.Start(stop)

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

// buildBrokerAdapter picks Alpaca's paper-trading adapter when API
// credentials are configured, otherwise falls back to a fully local
// PaperBroker driven by a synthetic random-walk price feed.
func buildBrokerAdapter(symbol domain.Symbol) broker.Adapter {
	keyID := os.Getenv("APCA_API_KEY_ID")
	secret := os.Getenv("APCA_API_SECRET_KEY")
	if keyID != "" && secret != "" {
		log.Println("using Alpaca paper trading adapter")
		return alpaca.NewPaperAdapter(keyID, secret)
	}

	log.Println("no Alpaca credentials found, using synthetic data + local paper broker")
	source := synthetic.New(time.Now().UnixNano())
	source.SetInitialPrice(symbol.Ticker, 150)
	return paper.New("local-paper", source, paper.DefaultConfig(10_000))
}
