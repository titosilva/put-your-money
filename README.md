# put-your-money

A trading strategy simulator: run strategies against live market data with
**no real money involved**, and see how they would have performed via a
dashboard. Built to scale to multiple strategies, multiple asset classes,
and non-price ("external") input signals, and to let the execution venue be
swapped without touching strategy code.

## Architecture

- `internal/domain` — shared types (`Symbol`, `MarketState`, `Order`, `Fill`,
  `Portfolio`, ...). No package below depends on anything outside this one.
- `internal/strategy` — the `Strategy` interface. A strategy only ever sees
  `MarketState` + its own `Portfolio` and emits `Order`s — it never talks to
  a broker directly.
  - `internal/indicators` — streaming technical indicators (EMA/SMA, Wilder's
    RSI, MACD, Bollinger Bands) shared by the strategies below.
  - `internal/strategy/movingaverage` — moving-average crossover.
  - `internal/strategy/rsi` — RSI mean reversion (Wilder, 1978): buy oversold
    (RSI < 30), sell overbought (RSI > 70).
  - `internal/strategy/macd` — MACD crossover (Appel): buy when the MACD line
    crosses above its signal line, sell on the cross back below.
  - `internal/strategy/bollinger` — Bollinger Bands mean reversion: buy when
    price drops below the lower band, sell on reversion to the middle band.
  - `internal/strategy/pairs` — pairs trading / statistical arbitrage
    ("distance method", Gatev, Goetzmann & Rouwenhorst, 2006): short the
    outperformer and long the underperformer when the spread between two
    normalized price series diverges beyond a threshold, close both legs on
    reversion to the mean. The first strategy trading more than one symbol,
    and the first needing short selling.
  - All five are classical, price-only strategies with no external input.
- `internal/broker` — the `Adapter` interface: `GetMarketState` +
  `SubmitOrder`. Every execution venue implements this once.
  - `internal/broker/paper` — a fully simulated broker (slippage + fee
    model, in-memory portfolio) that gets quotes from any `QuoteSource`.
    Supports long and short positions (no margin/borrow-cost modeling).
  - `internal/broker/synthetic` — a random-walk `QuoteSource`, so the whole
    system runs with zero external credentials for local development. Can
    also generate a "companion" ticker whose price mean-reverts around
    another's, so pairs trading has something to actually converge on.
  - `internal/broker/alpaca` — wraps Alpaca's Trading + Market Data API,
    hard-wired to Alpaca's **paper trading** endpoint. This is the adapter to
    swap out for a real venue (e.g. a Brazilian corretora) later.
- `internal/engine` — the `Run` type: ticks a `Strategy` against a
  `broker.Adapter` on an interval, persists fills/equity to a `storage.Store`.
- `internal/storage` — the `Store` interface for run history.
  - `internal/storage/memory` — in-memory implementation (swap for
    Postgres/SQLite later without touching engine or API code).
- `internal/api` — read-only HTTP API over `storage.Store` for the dashboard.
- `web/` — a minimal static dashboard (equity curve + fills table).
- `internal/backtest` — replays stored historical bars (`HistoricalSource`,
  a `paper.QuoteSource`) through a strategy at full speed (no wall-clock
  ticking), producing a `Result` (final equity, return %, max drawdown,
  trade count). This is the automated-benchmark path: every strategy, new
  or old, gets evaluated over real historical data in milliseconds instead
  of needing a live/synthetic run to play out over real time.
- `cmd/fetchdata` — one-off tool: pulls historical daily bars from Alpaca's
  Market Data API and writes them to `testdata/historical/<TICKER>.csv`.
  Run it again to refresh the data; needs `APCA_API_KEY_ID`/`APCA_API_SECRET_KEY`
  (a market-data-capable key, paper or live — market data access isn't
  paper/live-specific).
- `cmd/backtest` — CLI wrapping `internal/backtest`: runs every built-in
  strategy against `testdata/historical/` and prints a comparison table.
- `cmd/engine` — wires the live/synthetic path together and serves the
  dashboard.

The isolation that matters: **strategies never import broker packages, and
broker adapters never import strategy packages.** Adding a new strategy or a
new venue is a new file, not a change to the engine.

## Running

```sh
go run ./cmd/engine
```

Serves the dashboard at http://localhost:8080. With no configuration it uses
a synthetic random-walk price feed (AAPL, plus a synthetic MSFT companion
for the pairs strategy) and launches **all five built-in strategies at
once**, each with its own isolated `PaperBroker` portfolio but fed identical
prices, so the dashboard can compare them side by side.

To run against Alpaca's paper trading market data and simulated execution
instead, set your Alpaca **paper** API keys (this runs a single strategy,
since it trades against one real paper account — pick it with `STRATEGY`):

```sh
export APCA_API_KEY_ID=...
export APCA_API_SECRET_KEY=...
export STRATEGY=rsi   # ma | rsi | macd | bollinger | pairs (default: ma)
go run ./cmd/engine
```

No code path in this project submits an order anywhere except Alpaca's own
paper-trading endpoint or the fully local `PaperBroker` — there is no live
trading integration.

## Benchmarking strategies against historical data

`testdata/historical/*.csv` holds ~3 years of real daily close prices for
AAPL and MSFT (fetched once via `cmd/fetchdata`, split-adjusted, committed
to the repo so benchmarks are reproducible without network access or an
Alpaca account).

```sh
go run ./cmd/backtest
```

prints a table like:

```
STRATEGY                  FINAL EQUITY  TOTAL RETURN  MAX DRAWDOWN  TRADES
moving-average-crossover  $11026.07     10.26%        4.25%         184
rsi-mean-reversion        $10674.24     6.74%         7.05%         8
macd-crossover            $10572.20     5.72%         5.09%         59
bollinger-mean-reversion  $10655.06     6.55%         3.45%         26
pairs-trading             $10192.96     1.93%         10.93%        112
```

The same path also runs as `go test ./internal/backtest/...` (`-v` to see
the per-strategy numbers), so every strategy gets checked against real data
on every test run — a new strategy just needs adding to the `strategies`
slice in `cmd/backtest/main.go` and the test's slice alongside it.

To refresh or extend the historical data (a longer window, more tickers),
re-run `cmd/fetchdata` with an Alpaca key pair (paper keys are fine — market
data access isn't gated by paper vs. live) and commit the updated CSVs:

```sh
export APCA_API_KEY_ID=...
export APCA_API_SECRET_KEY=...
go run ./cmd/fetchdata
```

## Status / next steps

- Alpaca paper trading works for US equities; Brazil is not currently a
  supported country of tax residence for Alpaca's *live* trading, so a
  Brazilian corretora (or B3-facing) adapter is the intended path for a real
  production venue later — implementing `broker.Adapter` is all that takes.
- `storage.Store` should move to Postgres/SQLite once runs need to persist
  across restarts.
- External-input strategies (news sentiment, economic signals) are the
  natural next step, via `domain.Signal` in `MarketState` — no engine changes
  needed, just a `DataSource` producing signals and a strategy reading them.
- Pairs trading's hedge ratio here is fixed at 1:1 on normalized series (the
  original "distance method"); a regression-based hedge ratio (spread =
  A - β·B) would be a natural refinement for real (non-synthetic) pairs.
- `PaperBroker` short-selling has no margin requirement or borrow cost
  modeled — fine for comparing strategies' directional calls, but it means
  short-side P&L is optimistic vs. a real broker.
- The backtest results above are on daily bars over one ~3-year window with
  default strategy parameters — not a claim any strategy is profitable in
  general. Worth adding next: multiple historical windows/tickers per
  strategy, and parameter sweeps, before drawing conclusions from this.
