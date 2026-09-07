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

// companion ties one ticker's price to another's via a mean-reverting
// (Ornstein-Uhlenbeck-style) spread, so pairs-trading-style strategies have
// something to actually converge on in local demos — two fully independent
// random walks essentially never form a mean-reverting spread.
type companion struct {
	baseTicker     string
	ratio          float64
	reversionSpeed float64
	noiseVol       float64
	spread         float64
}

type Source struct {
	rng *rand.Rand

	mu         sync.Mutex
	prices     map[string]float64
	companions map[string]companion
}

func New(seed int64) *Source {
	return &Source{
		rng:        rand.New(rand.NewSource(seed)),
		prices:     make(map[string]float64),
		companions: make(map[string]companion),
	}
}

// SetInitialPrice seeds the starting price for a ticker; if never called,
// the first GetMarketState call for that ticker starts it at 100. Ignored
// for tickers set up via SetCompanion, whose price is always derived from
// their base ticker.
func (s *Source) SetInitialPrice(ticker string, price float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.prices[ticker] = price
}

// SetCompanion makes `ticker`'s price track baseTicker's price times ratio,
// plus a mean-reverting spread (reversionSpeed pulls it back to zero each
// tick, noiseVol is the size of the random shock). This produces a
// cointegrated-like pair, useful for exercising pairs-trading strategies
// without needing real correlated market data.
func (s *Source) SetCompanion(ticker, baseTicker string, ratio, reversionSpeed, noiseVol float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.companions[ticker] = companion{
		baseTicker:     baseTicker,
		ratio:          ratio,
		reversionSpeed: reversionSpeed,
		noiseVol:       noiseVol,
	}
}

func (s *Source) GetMarketState(symbols []domain.Symbol) (domain.MarketState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	updated := make(map[string]bool)

	// pass 1: independent random-walk tickers (anything not a companion).
	for _, sym := range symbols {
		if _, isCompanion := s.companions[sym.Ticker]; isCompanion {
			continue
		}
		s.stepIndependent(sym.Ticker, updated)
	}

	// pass 2: companions, derived from their (now up to date) base ticker.
	for _, sym := range symbols {
		cfg, ok := s.companions[sym.Ticker]
		if !ok {
			continue
		}
		basePrice := s.stepIndependent(cfg.baseTicker, updated)

		cfg.spread += cfg.reversionSpeed*(-cfg.spread) + cfg.noiseVol*s.rng.NormFloat64()
		s.companions[sym.Ticker] = cfg

		price := basePrice * cfg.ratio * (1 + cfg.spread)
		if price < 0.01 {
			price = 0.01
		}
		s.prices[sym.Ticker] = price
		updated[sym.Ticker] = true
	}

	quotes := make(map[string]domain.Quote, len(symbols))
	for _, sym := range symbols {
		quotes[sym.Ticker] = domain.Quote{Symbol: sym, Price: s.prices[sym.Ticker], Timestamp: now}
	}

	return domain.MarketState{
		Timestamp: now,
		Quotes:    quotes,
		Signals:   map[string]domain.Signal{},
	}, nil
}

// stepIndependent advances ticker's plain random walk by one tick, unless
// it's already been updated this call (e.g. as another symbol's base).
func (s *Source) stepIndependent(ticker string, updated map[string]bool) float64 {
	if updated[ticker] {
		return s.prices[ticker]
	}
	price, ok := s.prices[ticker]
	if !ok {
		price = 100
	}
	// small random step, roughly +/-0.5% per tick
	step := (s.rng.Float64() - 0.5) * 0.01 * price
	price += step
	if price < 0.01 {
		price = 0.01
	}
	s.prices[ticker] = price
	updated[ticker] = true
	return price
}
