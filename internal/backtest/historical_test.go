package backtest

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/titosilva/put-your-money/internal/domain"
)

func writeTestCSV(t *testing.T, dir, ticker string, rows [][2]string) {
	t.Helper()
	f, err := os.Create(filepath.Join(dir, ticker+".csv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	f.WriteString("timestamp,close\n")
	for _, row := range rows {
		f.WriteString(row[0] + "," + row[1] + "\n")
	}
}

func TestHistoricalSource_AlignsOnSharedDates(t *testing.T) {
	dir := t.TempDir()
	// AAPL has a bar on the 3rd that MSFT is missing: it must be dropped
	// from the replay so every tick has quotes for both symbols.
	writeTestCSV(t, dir, "AAPL", [][2]string{
		{"2024-01-01T00:00:00Z", "100"},
		{"2024-01-02T00:00:00Z", "101"},
		{"2024-01-03T00:00:00Z", "102"},
	})
	writeTestCSV(t, dir, "MSFT", [][2]string{
		{"2024-01-01T00:00:00Z", "200"},
		{"2024-01-02T00:00:00Z", "201"},
	})

	symA := domain.Symbol{Ticker: "AAPL"}
	symB := domain.Symbol{Ticker: "MSFT"}
	source, err := NewHistoricalSource(dir, symA, symB)
	if err != nil {
		t.Fatalf("NewHistoricalSource: %v", err)
	}
	if source.Len() != 2 {
		t.Fatalf("expected 2 aligned dates, got %d", source.Len())
	}

	state, err := source.GetMarketState([]domain.Symbol{symA, symB})
	if err != nil {
		t.Fatalf("GetMarketState: %v", err)
	}
	if q, ok := state.Quote("AAPL"); !ok || q.Price != 100 {
		t.Fatalf("expected AAPL=100, got %v (ok=%v)", q.Price, ok)
	}
	if q, ok := state.Quote("MSFT"); !ok || q.Price != 200 {
		t.Fatalf("expected MSFT=200, got %v (ok=%v)", q.Price, ok)
	}

	if _, err := source.GetMarketState([]domain.Symbol{symA, symB}); err != nil {
		t.Fatalf("second GetMarketState: %v", err)
	}
	if !source.Done() {
		t.Fatal("expected source to be exhausted after 2 ticks")
	}
	if _, err := source.GetMarketState([]domain.Symbol{symA, symB}); err == nil {
		t.Fatal("expected an error once the source is exhausted")
	}
}

func TestHistoricalSource_AttachSignal(t *testing.T) {
	dir := t.TempDir()
	writeTestCSV(t, dir, "AAPL", [][2]string{
		{"2024-01-01T00:00:00Z", "100"},
		{"2024-01-02T00:00:00Z", "101"},
		{"2024-01-03T00:00:00Z", "102"},
		{"2024-01-04T00:00:00Z", "103"},
	})

	sym := domain.Symbol{Ticker: "AAPL"}
	source, err := NewHistoricalSource(dir, sym)
	if err != nil {
		t.Fatalf("NewHistoricalSource: %v", err)
	}

	source.AttachSignal("AAPL:earnings_surprise_pct", []Event{
		// exact match on a trading day
		{Date: mustParseDate(t, "2024-01-02"), Value: 5.0},
		// falls on a day with no bar (weekend-ish gap): should snap to the
		// next trading day on or after it.
		{Date: mustParseDate(t, "2024-01-02T12:00:00Z"), Value: 9.0},
	})

	var gotDay2, gotDay1 bool
	for !source.Done() {
		state, err := source.GetMarketState([]domain.Symbol{sym})
		if err != nil {
			t.Fatalf("GetMarketState: %v", err)
		}
		switch state.Timestamp {
		case mustParseDate(t, "2024-01-02"):
			gotDay1 = true
			if sig, ok := state.Signals["AAPL:earnings_surprise_pct"]; !ok || sig.Value != 5.0 {
				t.Fatalf("expected signal 5.0 on 01-02, got %+v (ok=%v)", sig, ok)
			}
		case mustParseDate(t, "2024-01-03"):
			gotDay2 = true
			if sig, ok := state.Signals["AAPL:earnings_surprise_pct"]; !ok || sig.Value != 9.0 {
				t.Fatalf("expected signal 9.0 snapped to 01-03, got %+v (ok=%v)", sig, ok)
			}
		default:
			if _, ok := state.Signals["AAPL:earnings_surprise_pct"]; ok {
				t.Fatalf("did not expect a signal on %v", state.Timestamp)
			}
		}
	}
	if !gotDay1 || !gotDay2 {
		t.Fatalf("expected signals on both 01-02 (exact) and 01-03 (snapped), got day1=%v day2=%v", gotDay1, gotDay2)
	}
}

func mustParseDate(t *testing.T, s string) time.Time {
	t.Helper()
	for _, layout := range []string{"2006-01-02", time.RFC3339} {
		if ts, err := time.Parse(layout, s); err == nil {
			return ts
		}
	}
	t.Fatalf("could not parse date %q", s)
	return time.Time{}
}

func TestHistoricalSource_Reset(t *testing.T) {
	dir := t.TempDir()
	writeTestCSV(t, dir, "AAPL", [][2]string{
		{"2024-01-01T00:00:00Z", "100"},
		{"2024-01-02T00:00:00Z", "101"},
	})

	sym := domain.Symbol{Ticker: "AAPL"}
	source, err := NewHistoricalSource(dir, sym)
	if err != nil {
		t.Fatalf("NewHistoricalSource: %v", err)
	}

	for !source.Done() {
		if _, err := source.GetMarketState([]domain.Symbol{sym}); err != nil {
			t.Fatalf("GetMarketState: %v", err)
		}
	}
	source.Reset()
	if source.Done() {
		t.Fatal("expected source to be replayable again after Reset")
	}
	if _, err := source.GetMarketState([]domain.Symbol{sym}); err != nil {
		t.Fatalf("GetMarketState after reset: %v", err)
	}
}
