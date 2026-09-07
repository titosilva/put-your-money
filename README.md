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
  - All four are classical, price-only, single-asset strategies — no
    external input, no shorting (the `PaperBroker` is long/flat only for now).
- `internal/broker` — the `Adapter` interface: `GetMarketState` +
  `SubmitOrder`. Every execution venue implements this once.
  - `internal/broker/paper` — a fully simulated broker (slippage + fee
    model, in-memory portfolio) that gets quotes from any `QuoteSource`.
  - `internal/broker/synthetic` — a random-walk `QuoteSource`, so the whole
    system runs with zero external credentials for local development.
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
- `cmd/engine` — wires it all together and serves the dashboard.

The isolation that matters: **strategies never import broker packages, and
broker adapters never import strategy packages.** Adding a new strategy or a
new venue is a new file, not a change to the engine.

## Running

```sh
go run ./cmd/engine
```

Serves the dashboard at http://localhost:8080. With no configuration it uses
a synthetic random-walk price feed and launches **all four built-in
strategies at once**, each with its own isolated `PaperBroker` portfolio but
fed identical prices, so the dashboard can compare them side by side.

To run against Alpaca's paper trading market data and simulated execution
instead, set your Alpaca **paper** API keys (this runs a single strategy,
since it trades against one real paper account — pick it with `STRATEGY`):

```sh
export APCA_API_KEY_ID=...
export APCA_API_SECRET_KEY=...
export STRATEGY=rsi   # ma | rsi | macd | bollinger (default: ma)
go run ./cmd/engine
```

No code path in this project submits an order anywhere except Alpaca's own
paper-trading endpoint or the fully local `PaperBroker` — there is no live
trading integration.

## Status / next steps

- Alpaca paper trading works for US equities; Brazil is not currently a
  supported country of tax residence for Alpaca's *live* trading, so a
  Brazilian corretora (or B3-facing) adapter is the intended path for a real
  production venue later — implementing `broker.Adapter` is all that takes.
- `storage.Store` should move to Postgres/SQLite once runs need to persist
  across restarts.
- Next classical strategy worth adding is pairs trading / statistical
  arbitrage (Gatev, Goetzmann & Rouwenhorst) — the first genuinely
  multi-asset one — but it needs short-selling, so it's blocked on extending
  `PaperBroker` to support negative (short) positions first.
- External-input strategies (news sentiment, economic signals) come after
  the classical set, via `domain.Signal` in `MarketState` — no engine changes
  needed, just a `DataSource` producing signals and a strategy reading them.
