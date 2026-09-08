// Command fetchearnings pulls historical earnings-surprise data from
// Finnhub's free /stock/earnings endpoint and writes it as CSV under
// testdata/historical/, one file per ticker — the event-data counterpart
// to cmd/fetchdata's price CSVs. internal/strategy/pead reads these files
// (via internal/backtest's signal attachment) to trade on earnings drift.
//
// IMPORTANT LIMITATION: Finnhub's free tier caps this endpoint to the last
// 4 quarters, ignoring any limit/from/to parameters — there is no way to
// get deeper free history from Finnhub for this data. That means the PEAD
// backtest only has signal for the most recent ~1 year of the 3-year price
// window in testdata/historical/*.csv, not the whole period. See
// LITERATURE.md and README.md for the consequences of that.
//
// A second approximation: Finnhub's free tier does not return the actual
// earnings announcement date, only the fiscal quarter-end ("period"). This
// tool approximates the announcement date as period + reportLagDays, a
// typical reporting lag — genuinely approximate, not the real date.
//
// Requires APCA-unrelated credentials: FINNHUB_API_KEY in the environment.
package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"
)

var tickers = []string{"AAPL", "MSFT"}

// reportLagDays approximates how long after a fiscal quarter end a company
// typically announces earnings. ~25 calendar days is a reasonable average
// for large-cap US companies; real lags vary by a couple of weeks either
// way, which is exactly why this is documented as an approximation rather
// than baked in silently.
const reportLagDays = 25

type finnhubEarning struct {
	Symbol          string  `json:"symbol"`
	Estimate        float64 `json:"estimate"`
	Actual          float64 `json:"actual"`
	Period          string  `json:"period"` // fiscal quarter end, YYYY-MM-DD
	Surprise        float64 `json:"surprise"`
	SurprisePercent float64 `json:"surprisePercent"`
}

func main() {
	apiKey := os.Getenv("FINNHUB_API_KEY")
	if apiKey == "" {
		log.Fatal("FINNHUB_API_KEY must be set")
	}

	if err := os.MkdirAll("testdata/historical", 0o755); err != nil {
		log.Fatalf("creating testdata/historical: %v", err)
	}

	for _, ticker := range tickers {
		earnings, err := fetchEarnings(ticker, apiKey)
		if err != nil {
			log.Fatalf("fetching earnings for %s: %v", ticker, err)
		}
		if len(earnings) == 0 {
			log.Fatalf("no earnings data returned for %s", ticker)
		}

		path := fmt.Sprintf("testdata/historical/%s_earnings.csv", ticker)
		if err := writeCSV(path, earnings); err != nil {
			log.Fatalf("writing %s: %v", path, err)
		}
		log.Printf("wrote %d earnings events for %s to %s", len(earnings), ticker, path)
	}
}

func fetchEarnings(ticker, apiKey string) ([]finnhubEarning, error) {
	u := "https://finnhub.io/api/v1/stock/earnings?" + url.Values{
		"symbol": {ticker},
		"token":  {apiKey},
	}.Encode()

	resp, err := http.Get(u)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("finnhub returned %s: %s", resp.Status, body)
	}

	var earnings []finnhubEarning
	if err := json.NewDecoder(resp.Body).Decode(&earnings); err != nil {
		return nil, err
	}
	return earnings, nil
}

func writeCSV(path string, earnings []finnhubEarning) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()

	if err := w.Write([]string{"period", "approx_report_date", "estimate", "actual", "surprise", "surprise_percent"}); err != nil {
		return err
	}
	for _, e := range earnings {
		periodDate, err := time.Parse("2006-01-02", e.Period)
		if err != nil {
			return fmt.Errorf("parsing period %q: %w", e.Period, err)
		}
		approxReportDate := periodDate.AddDate(0, 0, reportLagDays)

		record := []string{
			e.Period,
			approxReportDate.Format("2006-01-02"),
			strconv.FormatFloat(e.Estimate, 'f', -1, 64),
			strconv.FormatFloat(e.Actual, 'f', -1, 64),
			strconv.FormatFloat(e.Surprise, 'f', -1, 64),
			strconv.FormatFloat(e.SurprisePercent, 'f', -1, 64),
		}
		if err := w.Write(record); err != nil {
			return err
		}
	}
	return nil
}
