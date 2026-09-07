// Package indicators implements the streaming technical indicators used by
// the classical strategies: EMA/SMA, Wilder's RSI, MACD, and Bollinger
// Bands. Each type consumes one price at a time via Update, so a strategy
// can feed it ticks without keeping its own history.
package indicators

import "math"

// EMA is an exponential moving average. The first value seeds the average
// (a common simplification vs. SMA-seeding); Ready reports once at least
// `period` updates have been seen.
type EMA struct {
	period int
	alpha  float64
	value  float64
	count  int
}

func NewEMA(period int) *EMA {
	return &EMA{period: period, alpha: 2 / (float64(period) + 1)}
}

func (e *EMA) Update(price float64) float64 {
	e.count++
	if e.count == 1 {
		e.value = price
	} else {
		e.value = e.alpha*price + (1-e.alpha)*e.value
	}
	return e.value
}

func (e *EMA) Value() float64 { return e.value }
func (e *EMA) Ready() bool    { return e.count >= e.period }

// SMA is a simple moving average over the last `period` prices.
type SMA struct {
	period int
	window []float64
}

func NewSMA(period int) *SMA {
	return &SMA{period: period, window: make([]float64, 0, period)}
}

// Update appends price and returns the current average and whether the
// window is full yet.
func (s *SMA) Update(price float64) (value float64, ready bool) {
	s.window = append(s.window, price)
	if len(s.window) > s.period {
		s.window = s.window[len(s.window)-s.period:]
	}
	if len(s.window) < s.period {
		return 0, false
	}
	return mean(s.window), true
}

// RSI is Wilder's Relative Strength Index (Wilder, 1978): compares average
// gains to average losses over `period` price changes to gauge overbought
// (>70) / oversold (<30) conditions.
type RSI struct {
	period    int
	prevPrice float64
	hasPrev   bool

	sumGain, sumLoss float64 // accumulated during warm-up
	avgGain, avgLoss float64
	seenChanges      int
	ready            bool
}

func NewRSI(period int) *RSI {
	return &RSI{period: period}
}

func (r *RSI) Update(price float64) (value float64, ready bool) {
	if !r.hasPrev {
		r.prevPrice = price
		r.hasPrev = true
		return 0, false
	}

	diff := price - r.prevPrice
	r.prevPrice = price
	gain := math.Max(diff, 0)
	loss := math.Max(-diff, 0)
	r.seenChanges++

	switch {
	case r.seenChanges < r.period:
		r.sumGain += gain
		r.sumLoss += loss
		return 0, false
	case r.seenChanges == r.period:
		r.sumGain += gain
		r.sumLoss += loss
		r.avgGain = r.sumGain / float64(r.period)
		r.avgLoss = r.sumLoss / float64(r.period)
		r.ready = true
	default:
		r.avgGain = (r.avgGain*float64(r.period-1) + gain) / float64(r.period)
		r.avgLoss = (r.avgLoss*float64(r.period-1) + loss) / float64(r.period)
	}

	if r.avgLoss == 0 {
		return 100, true
	}
	rs := r.avgGain / r.avgLoss
	return 100 - 100/(1+rs), true
}

// MACD is Moving Average Convergence Divergence (Appel): the difference
// between a fast and slow EMA, plus an EMA of that difference (the "signal
// line"). Crossovers between MACD and signal mark momentum shifts.
type MACD struct {
	fast, slow, signal *EMA
}

func NewMACD(fastPeriod, slowPeriod, signalPeriod int) *MACD {
	return &MACD{
		fast:   NewEMA(fastPeriod),
		slow:   NewEMA(slowPeriod),
		signal: NewEMA(signalPeriod),
	}
}

func (m *MACD) Update(price float64) (macd, signal float64, ready bool) {
	fastVal := m.fast.Update(price)
	slowVal := m.slow.Update(price)
	macdVal := fastVal - slowVal
	signalVal := m.signal.Update(macdVal)
	ready = m.slow.Ready() && m.signal.Ready()
	return macdVal, signalVal, ready
}

// Bollinger computes Bollinger Bands (Bollinger): a moving average with
// upper/lower bands `k` standard deviations away, used to flag price
// extremes relative to recent volatility.
type Bollinger struct {
	period int
	k      float64
	window []float64
}

func NewBollinger(period int, k float64) *Bollinger {
	return &Bollinger{period: period, k: k, window: make([]float64, 0, period)}
}

func (b *Bollinger) Update(price float64) (mid, upper, lower float64, ready bool) {
	b.window = append(b.window, price)
	if len(b.window) > b.period {
		b.window = b.window[len(b.window)-b.period:]
	}
	if len(b.window) < b.period {
		return 0, 0, 0, false
	}
	mid = mean(b.window)
	sd := stddev(b.window, mid)
	return mid, mid + b.k*sd, mid - b.k*sd, true
}

func mean(xs []float64) float64 {
	var sum float64
	for _, x := range xs {
		sum += x
	}
	return sum / float64(len(xs))
}

func stddev(xs []float64, mean float64) float64 {
	var sumSq float64
	for _, x := range xs {
		d := x - mean
		sumSq += d * d
	}
	return math.Sqrt(sumSq / float64(len(xs)))
}
