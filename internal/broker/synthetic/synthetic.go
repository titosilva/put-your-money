// Package synthetic provides a random-walk price generator implementing
// paper.QuoteSource, so the engine can be developed and tested end-to-end
// without needing real broker credentials. Swap it for the Alpaca adapter's
// data feed (or any other QuoteSource) to run against real market data.
package synthetic

import (
	"math/rand"
	"sync"
	"time"

	"github.com/titosilva/put-your-money/internal/domain"
)

type Source struct {
	rng *rand.Rand

	mu     sync.Mutex
	prices map[string]float64
}

func New(seed int64) *Source {
	return &Source{
		rng:    rand.New(rand.NewSource(seed)),
		prices: make(map[string]float64),
	}
}

// SetInitialPrice seeds the starting price for a ticker; if never called,
// the first GetMarketState call for that ticker starts it at 100.
func (s *Source) SetInitialPrice(ticker string, price float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.prices[ticker] = price
}

func (s *Source) GetMarketState(symbols []domain.Symbol) (domain.MarketState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	quotes := make(map[string]domain.Quote, len(symbols))
	for _, sym := range symbols {
		price, ok := s.prices[sym.Ticker]
		if !ok {
			price = 100
		}
		// small random step, roughly +/-0.5% per tick
		step := (s.rng.Float64() - 0.5) * 0.01 * price
		price += step
		if price < 0.01 {
			price = 0.01
		}
		s.prices[sym.Ticker] = price

		quotes[sym.Ticker] = domain.Quote{Symbol: sym, Price: price, Timestamp: now}
	}

	return domain.MarketState{
		Timestamp: now,
		Quotes:    quotes,
		Signals:   map[string]domain.Signal{},
	}, nil
}
