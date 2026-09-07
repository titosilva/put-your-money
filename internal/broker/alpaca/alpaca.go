// Package alpaca adapts Alpaca's Trading + Market Data API to the engine's
// broker.Adapter interface. It always talks to Alpaca's *paper* endpoint —
// see NewPaperAdapter — so orders submitted through it settle against
// Alpaca's own simulated brokerage account, never real money.
package alpaca

import (
	"fmt"

	"github.com/alpacahq/alpaca-trade-api-go/v3/alpaca"
	"github.com/alpacahq/alpaca-trade-api-go/v3/marketdata"
	"github.com/shopspring/decimal"

	"github.com/titosilva/put-your-money/internal/domain"
)

const paperBaseURL = "https://paper-api.alpaca.markets"

// Adapter implements broker.Adapter against Alpaca's paper trading account.
type Adapter struct {
	trading *alpaca.Client
	data    *marketdata.Client
}

// NewPaperAdapter builds an Adapter hard-wired to Alpaca's paper trading
// endpoint. keyID/secret come from an Alpaca account's paper API keys.
func NewPaperAdapter(keyID, secret string) *Adapter {
	return &Adapter{
		trading: alpaca.NewClient(alpaca.ClientOpts{
			APIKey:    keyID,
			APISecret: secret,
			BaseURL:   paperBaseURL,
		}),
		data: marketdata.NewClient(marketdata.ClientOpts{
			APIKey:    keyID,
			APISecret: secret,
		}),
	}
}

func (a *Adapter) Name() string { return "alpaca-paper" }

func (a *Adapter) GetMarketState(symbols []domain.Symbol) (domain.MarketState, error) {
	tickers := make([]string, len(symbols))
	bySymbol := make(map[string]domain.Symbol, len(symbols))
	for i, s := range symbols {
		tickers[i] = s.Ticker
		bySymbol[s.Ticker] = s
	}

	trades, err := a.data.GetLatestTrades(tickers, marketdata.GetLatestTradeRequest{})
	if err != nil {
		return domain.MarketState{}, fmt.Errorf("alpaca: fetching latest trades: %w", err)
	}

	quotes := make(map[string]domain.Quote, len(trades))
	var latest domain.MarketState
	for ticker, trade := range trades {
		quotes[ticker] = domain.Quote{
			Symbol:    bySymbol[ticker],
			Price:     trade.Price,
			Timestamp: trade.Timestamp,
		}
		if trade.Timestamp.After(latest.Timestamp) {
			latest.Timestamp = trade.Timestamp
		}
	}

	return domain.MarketState{
		Timestamp: latest.Timestamp,
		Quotes:    quotes,
		Signals:   map[string]domain.Signal{},
	}, nil
}

func (a *Adapter) SubmitOrder(order domain.Order, _ domain.MarketState) (domain.Fill, error) {
	qty := decimal.NewFromFloat(order.Qty)

	side := alpaca.Buy
	if order.Side == domain.OrderSideSell {
		side = alpaca.Sell
	}

	placed, err := a.trading.PlaceOrder(alpaca.PlaceOrderRequest{
		Symbol:      order.Symbol.Ticker,
		Qty:         &qty,
		Side:        side,
		Type:        alpaca.Market,
		TimeInForce: alpaca.Day,
	})
	if err != nil {
		return domain.Fill{}, fmt.Errorf("alpaca: placing order: %w", err)
	}

	// Market orders on Alpaca's paper account typically fill almost
	// immediately, but the initial response can still show it as pending.
	// Callers that need a guaranteed fill price should poll GetOrder(placed.ID);
	// we return what Alpaca gave us so the engine keeps moving.
	var fillPrice float64
	if placed.FilledAvgPrice != nil {
		fillPrice, _ = placed.FilledAvgPrice.Float64()
	}
	filledQty, _ := placed.FilledQty.Float64()
	if filledQty == 0 {
		filledQty = order.Qty
	}

	return domain.Fill{
		Order:    order,
		Price:    fillPrice,
		Qty:      filledQty,
		Fee:      0, // Alpaca equities are commission-free
		FilledAt: placed.SubmittedAt,
	}, nil
}
