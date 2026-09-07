# Literature notes

Academic context for the strategies in this repo, gathered so future
strategy work starts from what the research already knows rather than
re-discovering it. Not a literature review in the formal sense — a working
reference, meant to be extended as new strategies get added.

## Survey / review papers

- **Park & Irwin (2007), "The Profitability of Technical Analysis: A
  Review."** The classic survey of technical-analysis backtests. Across 92
  studies (1988–2004): 58 found positive excess profits, 10 mixed, 24
  losses; a broader later tally found 56 of 95 modern studies positive.
  Core caveat: results are riddled with **data-snooping bias**, and most
  positive results predate the late 1980s.
  https://farmdoc.illinois.edu/assets/marketing/agmas/AgMAS04_04.pdf
- **"Automated trading systems statistical and machine learning methods and
  hardware implementation: a survey"** (Taylor & Francis, 2018). Groups
  trading-system research into technical analysis, textual/sentiment
  analysis, and high-frequency trading, evaluating each empirically.
  https://www.tandfonline.com/doi/full/10.1080/17517575.2018.1493145
- **"Deep Reinforcement Learning for Trading — A Critical Survey"** (MDPI,
  2021). Surveys DRL applied to trading, cataloguing common architectures
  and failure modes. https://www.mdpi.com/2306-5729/6/11/119
- **"Reinforcement Learning in Algorithmic Trading: A Survey"** (2024).
  https://www.researchgate.net/publication/380571013_Reinforcement_Learning_in_Algorithmic_Trading_A_Survey
- **"Deep learning for algorithmic trading: A systematic review of
  predictive models and optimization strategies"** (2025).
  https://www.sciencedirect.com/science/article/pii/S2590005625000177
- **"Systematic Review on Algorithmic Trading"** (2025). PRISMA-guided
  review screening 1,567 articles across IEEE Xplore, ACM Digital Library,
  SpringerLink, Web of Science, and SSRN.
  https://www.researchgate.net/publication/394622553_Systematic_Review_on_Algorithmic_Trading

## What the literature says about specific strategy families

### Classical technical rules (moving averages, RSI, Bollinger Bands)

Park & Irwin's verdict: mixed and shrinking. Profitable in US equities up
to the late 1980s, then largely not; FX and futures markets kept showing
profitability longer, plausibly because trending regimes are more
persistent there than in equities. This directly covers
`internal/strategy/movingaverage`, `rsi`, `bollinger`, and `macd` here.

### Momentum

**Jegadeesh & Titman (1993)** — long winners / short losers on 3–12 month
lookbacks returned ~1.5%/month in the original 1963–1990 US sample. One of
the most robust anomalies in finance: replicated across 40+ countries,
a dozen+ asset classes, and traced back to 1801 in the US. Rare among
"classical" strategies in that it has held up well out-of-sample and
post-publication (see "Momentum: what do we know 30 years after Jegadeesh
and Titman's seminal paper?", 2022,
https://link.springer.com/article/10.1007/s11408-022-00417-8). Not yet
implemented here — the closest analog is `movingaverage`, which is trend-
following but on a much shorter horizon than academic momentum studies use.

### Pairs trading / statistical arbitrage

**Gatev, Goetzmann & Rouwenhorst (2006), "Pairs Trading: Performance of a
Relative Value Arbitrage Rule"** — the paper `internal/strategy/pairs`
implements (the "distance method"). Original 1962–1997 sample: ~11–12%
annualized excess returns, exceeding conservative transaction-cost
estimates. https://www.nber.org/papers/w7032

This is the strategy family with the clearest **documented decay**:
follow-up studies find its monthly alpha fell from ~0.86% to ~0.24% across
five decades, steepest drop after 2002. Attribution work splits the cause
roughly 70% "arbitrage risk got worse" (synchronization risk, noise-trader
risk) vs. 30% "pure crowding/efficiency."
https://www.researchgate.net/publication/271539832_On_the_determinants_of_pairs_trading_profitability

Directly relevant to this repo's own numbers: `cmd/backtest` shows
`pairs-trading` returning ~2% with the highest drawdown of the five
strategies over the 2023–2026 AAPL/MSFT window — entirely consistent with
what the literature would predict for this strategy in a modern sample.

### ML / deep reinforcement learning

The surveys converge on a warning more than a "it works" result: DRL
agents routinely **memorize the training regime** and fail to transfer
out-of-sample; reported performance is highly sensitive to reward
specification, state representation, and whether transaction costs/slippage
are modeled at all. Proposed mitigations (validation-set checkpointing,
ensembling, rolling retraining, hypothesis-testing for overfitting with 16+
splits) are themselves active research, not solved problems. Nothing in
this repo uses ML yet; if that changes, treat any backtest result on a
single historical window as close to meaningless until validated the way
these surveys describe.

## The cross-cutting finding

**McLean & Pontiff (2016), "Does Academic Research Destroy Stock Return
Predictability?"** — studied 97 published return predictors: returns are on
average **26% lower out-of-sample** and **58% lower after publication**,
consistent with sophisticated traders arbitraging away mispricings once
publicized. https://www.hec.ca/finance/Fichier/McLean.pdf

This "anomaly decay" (aka "shrinking alpha") is the single most important
finding for this project: a strategy's number on one backtest window is not
a claim about future performance, and the more well-known/published the
strategy, the more that gap should be expected to widen. It's the academic
version of the caveat already in the README's "Status / next steps."

## Practical implication for this repo

The `cmd/backtest` approach (§ README) is the right instinct, but one
number from one window is exactly the kind of result the decay literature
says not to trust. The natural extension, consistent with how the decay
studies above actually measure decay: run the same strategy across
**multiple historical windows/regimes** (different date ranges, different
tickers/sectors, bull vs. bear periods) and look at how its return moves
over time and across regimes, rather than reporting a single figure.
