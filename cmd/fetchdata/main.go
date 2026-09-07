// Command fetchdata pulls historical daily bars from Alpaca's Market Data
// API and writes them as CSV under testdata/historical/, one file per
// ticker. This is a one-off developer tool, not something the engine runs —
// the resulting CSVs are what internal/backtest actually reads.
//
// Requires APCA_API_KEY_ID / APCA_API_SECRET_KEY in the environment (an
// Alpaca paper-trading key pair is sufficient; market data access isn't
// tied to paper vs. live).
package main

import (
	"encoding/csv"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/alpacahq/alpaca-trade-api-go/v3/marketdata"
)

// tickers to fetch. Keep in sync with the symbols cmd/engine and
// internal/backtest expect to find under testdata/historical/.
var tickers = []string{"AAPL", "MSFT"}

func main() {
	keyID := os.Getenv("APCA_API_KEY_ID")
	secret := os.Getenv("APCA_API_SECRET_KEY")
	if keyID == "" || secret == "" {
		log.Fatal("APCA_API_KEY_ID and APCA_API_SECRET_KEY must be set")
	}

	client := marketdata.NewClient(marketdata.ClientOpts{
		APIKey:    keyID,
		APISecret: secret,
	})

	end := time.Now().AddDate(0, 0, -1) // yesterday: avoid asking for today's incomplete bar
	start := end.AddDate(-3, 0, 0)      // ~3 years of daily bars

	if err := os.MkdirAll("testdata/historical", 0o755); err != nil {
		log.Fatalf("creating testdata/historical: %v", err)
	}

	for _, ticker := range tickers {
		bars, err := client.GetBars(ticker, marketdata.GetBarsRequest{
			TimeFrame:  marketdata.OneDay,
			Adjustment: marketdata.AdjustmentSplit,
			Start:      start,
			End:        end,
			Feed:       marketdata.IEX, // available on Alpaca's free data plan
		})
		if err != nil {
			log.Fatalf("fetching bars for %s: %v", ticker, err)
		}
		if len(bars) == 0 {
			log.Fatalf("no bars returned for %s", ticker)
		}

		path := fmt.Sprintf("testdata/historical/%s.csv", ticker)
		if err := writeCSV(path, bars); err != nil {
			log.Fatalf("writing %s: %v", path, err)
		}
		log.Printf("wrote %d bars for %s to %s (%s to %s)", len(bars), ticker, path,
			bars[0].Timestamp.Format("2006-01-02"), bars[len(bars)-1].Timestamp.Format("2006-01-02"))
	}
}

func writeCSV(path string, bars []marketdata.Bar) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()

	if err := w.Write([]string{"timestamp", "close"}); err != nil {
		return err
	}
	for _, bar := range bars {
		record := []string{
			bar.Timestamp.UTC().Format(time.RFC3339),
			strconv.FormatFloat(bar.Close, 'f', -1, 64),
		}
		if err := w.Write(record); err != nil {
			return err
		}
	}
	return nil
}
