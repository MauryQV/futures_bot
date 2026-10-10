package strategy

import (
	"fmt"

	"futures_bot/internal/binance"
)

// EMA calcula la media móvil exponencial (semilla = primer valor).
func EMA(v []float64, n int) []float64 {
	out := make([]float64, len(v))
	if len(v) == 0 {
		return out
	}
	k := 2 / float64(n+1)
	out[0] = v[0]
	for i := 1; i < len(v); i++ {
		out[i] = v[i]*k + out[i-1]*(1-k)
	}
	return out
}

// RSI de Wilder; devuelve 50 si no hay datos suficientes.
func RSI(closes []float64, n int) float64 {
	if len(closes) <= n {
		return 50
	}
	var gain, loss float64
	for i := 1; i <= n; i++ {
		if d := closes[i] - closes[i-1]; d > 0 {
			gain += d
		} else {
			loss -= d
		}
	}
	fn := float64(n)
	gain, loss = gain/fn, loss/fn
	for i := n + 1; i < len(closes); i++ {
		d := closes[i] - closes[i-1]
		g, l := 0.0, 0.0
		if d > 0 {
			g = d
		} else {
			l = -d
		}
		gain, loss = (gain*(fn-1)+g)/fn, (loss*(fn-1)+l)/fn
	}
	if loss == 0 {
		return 100
	}
	return 100 - 100/(1+gain/loss)
}

// Levels devuelve soporte (mínimo) y resistencia (máximo) de las últimas
// `lookback` velas.
func Levels(c []binance.Candle, lookback int) (support, resistance float64) {
	if len(c) == 0 {
		return 0, 0
	}
	if lookback > len(c) {
		lookback = len(c)
	}
	w := c[len(c)-lookback:]
	support, resistance = w[0].Low, w[0].High
	for _, k := range w {
		if k.Low < support {
			support = k.Low
		}
		if k.High > resistance {
			resistance = k.High
		}
	}
	return
}

// LevelBlocked evita entrar "contra la pared": un long se bloquea si hay una
// resistencia por encima a menos de bufferPct %; un short, si hay un soporte
// por debajo a menos de bufferPct %.
func LevelBlocked(side string, price, support, resistance, bufferPct float64) (bool, string) {
	if bufferPct <= 0 || price <= 0 {
		return false, ""
	}
	switch side {
	case "long":
		if resistance > price {
			if d := (resistance - price) / price * 100; d < bufferPct {
				return true, fmt.Sprintf("resistencia %.2f a solo %.2f%% (buffer %.2f%%)", resistance, d, bufferPct)
			}
		}
	case "short":
		if support > 0 && support < price {
			if d := (price - support) / price * 100; d < bufferPct {
				return true, fmt.Sprintf("soporte %.2f a solo %.2f%% (buffer %.2f%%)", support, d, bufferPct)
			}
		}
	}
	return false, ""
}
