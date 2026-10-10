// Package strategy decide cuándo entrar. Hay dos decisores: EMACross (reglas) y
// LLM (modelo de lenguaje). Ambos son solo una *propuesta*: el bot aplica después
// sus propios filtros de riesgo.
package strategy

import (
	"context"

	"futures_bot/internal/binance"
)

// Snapshot es lo que ve el decisor en cada vela cerrada.
type Snapshot struct {
	Symbol, Interval    string
	Price               float64
	Candles             []binance.Candle // cerradas, de antigua a reciente
	Support, Resistance float64
	TPPct, SLPct        float64
	Leverage            int
}

// Decision: Side "long" | "short" | "" (nada). Confidence 0-100.
type Decision struct {
	Side       string
	Confidence int
	Reason     string
}

type Decider interface {
	Decide(ctx context.Context, s Snapshot) (Decision, error)
	Need() int // velas cerradas mínimas que necesita
}

// EMACross: long cuando la EMA rápida cruza por encima de la lenta; short al revés.
type EMACross struct{ Fast, Slow int }

func (s EMACross) Need() int { return s.Slow * 4 }

func (s EMACross) Decide(_ context.Context, sn Snapshot) (Decision, error) {
	closes := make([]float64, len(sn.Candles))
	for i, k := range sn.Candles {
		closes[i] = k.Close
	}
	side := s.Signal(closes)
	if side == "" {
		return Decision{Reason: "sin cruce de EMAs"}, nil
	}
	return Decision{Side: side, Confidence: 100, Reason: "cruce de EMAs"}, nil
}

// Signal devuelve "long", "short" o "" a partir de cierres de velas cerradas.
func (s EMACross) Signal(closes []float64) string {
	if len(closes) < s.Need() {
		return ""
	}
	f, sl := EMA(closes, s.Fast), EMA(closes, s.Slow)
	n := len(closes)
	f0, s0, f1, s1 := f[n-2], sl[n-2], f[n-1], sl[n-1]
	switch {
	case f0 <= s0 && f1 > s1:
		return "long"
	case f0 >= s0 && f1 < s1:
		return "short"
	}
	return ""
}
