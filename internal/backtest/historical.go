// Package backtest lets strategies run against stored historical price data
// instead of live ticks, so every strategy (existing or future) can be
// benchmarked in milliseconds instead of waiting out a real trading day.
package backtest

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/titosilva/put-your-money/internal/domain"
)

// bar is one row of stored historical data for a ticker.
type bar struct {
	Timestamp time.Time
	Close     float64
}

// LoadCSV reads a "timestamp,close" CSV (as written by cmd/fetchdata) for a
// single ticker.
func LoadCSV(path string) ([]bar, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("backtest: opening %s: %w", path, err)
	}
	defer f.Close()

	r := csv.NewReader(f)
	rows, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("backtest: reading %s: %w", path, err)
	}
	if len(rows) < 2 {
		return nil, fmt.Errorf("backtest: %s has no data rows", path)
	}

	bars := make([]bar, 0, len(rows)-1)
	for _, row := range rows[1:] { // skip header
		ts, err := time.Parse(time.RFC3339, row[0])
		if err != nil {
			return nil, fmt.Errorf("backtest: parsing timestamp %q in %s: %w", row[0], path, err)
		}
		var closePrice float64
		if _, err := fmt.Sscanf(row[1], "%g", &closePrice); err != nil {
			return nil, fmt.Errorf("backtest: parsing close %q in %s: %w", row[1], path, err)
		}
		bars = append(bars, bar{Timestamp: ts, Close: closePrice})
	}
	return bars, nil
}

// HistoricalSource replays stored bars for one or more symbols in lockstep,
// one row per call to GetMarketState — it implements paper.QuoteSource, so
// it's a drop-in replacement for the synthetic or Alpaca-backed data feeds
// wherever a strategy just needs a stream of MarketStates.
type HistoricalSource struct {
	symbols map[string]domain.Symbol
	bars    map[string][]bar
	dates   []time.Time // union of all dates across symbols, sorted ascending
	cursor  int
}

// NewHistoricalSource loads testdata/historical/<ticker>.csv for each given
// symbol and aligns them on their shared trading dates (a date missing for
// any symbol, e.g. a stock-specific halt, is skipped for all of them so
// every tick has quotes for every symbol).
func NewHistoricalSource(dataDir string, symbols ...domain.Symbol) (*HistoricalSource, error) {
	bySymbol := make(map[string][]bar, len(symbols))
	symbolByTicker := make(map[string]domain.Symbol, len(symbols))

	for _, sym := range symbols {
		path := filepath.Join(dataDir, sym.Ticker+".csv")
		bars, err := LoadCSV(path)
		if err != nil {
			return nil, err
		}
		bySymbol[sym.Ticker] = bars
		symbolByTicker[sym.Ticker] = sym
	}

	dateSets := make([]map[time.Time]float64, 0, len(symbols))
	for _, sym := range symbols {
		set := make(map[time.Time]float64, len(bySymbol[sym.Ticker]))
		for _, b := range bySymbol[sym.Ticker] {
			set[b.Timestamp] = b.Close
		}
		dateSets = append(dateSets, set)
	}

	var shared []time.Time
	for date := range dateSets[0] {
		inAll := true
		for _, set := range dateSets[1:] {
			if _, ok := set[date]; !ok {
				inAll = false
				break
			}
		}
		if inAll {
			shared = append(shared, date)
		}
	}
	sort.Slice(shared, func(i, j int) bool { return shared[i].Before(shared[j]) })

	if len(shared) == 0 {
		return nil, fmt.Errorf("backtest: no overlapping dates across symbols %v", symbols)
	}

	return &HistoricalSource{
		symbols: symbolByTicker,
		bars:    bySymbol,
		dates:   shared,
	}, nil
}

// Len returns the number of ticks available to replay.
func (s *HistoricalSource) Len() int { return len(s.dates) }

// Reset rewinds the replay cursor to the beginning without re-reading the
// CSVs, so the same loaded data can be re-run for each strategy benchmarked.
func (s *HistoricalSource) Reset() { s.cursor = 0 }

// Done reports whether every stored bar has already been replayed.
func (s *HistoricalSource) Done() bool { return s.cursor >= len(s.dates) }

// GetMarketState returns the next bar's quotes for the requested symbols
// and advances the internal cursor. Symbols not tracked by this source
// (i.e. no matching CSV was loaded) are simply omitted from the result,
// same as a real feed that has no data for a ticker.
func (s *HistoricalSource) GetMarketState(symbols []domain.Symbol) (domain.MarketState, error) {
	if s.Done() {
		return domain.MarketState{}, fmt.Errorf("backtest: historical source exhausted (%d bars replayed)", len(s.dates))
	}

	date := s.dates[s.cursor]
	s.cursor++

	quotes := make(map[string]domain.Quote, len(symbols))
	for _, sym := range symbols {
		bars, ok := s.bars[sym.Ticker]
		if !ok {
			continue
		}
		for _, b := range bars {
			if b.Timestamp.Equal(date) {
				quotes[sym.Ticker] = domain.Quote{Symbol: sym, Price: b.Close, Timestamp: date}
				break
			}
		}
	}

	return domain.MarketState{
		Timestamp: date,
		Quotes:    quotes,
		Signals:   map[string]domain.Signal{},
	}, nil
}
