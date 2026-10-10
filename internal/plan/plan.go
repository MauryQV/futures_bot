package plan

import (
	"fmt"

	"futures_bot/internal/binance"
	"futures_bot/internal/config"
)

type Plan struct {
	Symbol, Side, OrderType, MarginType string
	EntrySide, ExitSide                 string // BUY/SELL
	Entry, Qty, TP, SL                  float64
	TPPct, SLPct, Margin                float64
	Leverage                            int
	Rules                               *binance.Rules

	dir float64 // +1 long, -1 short
}

// Build arma el plan a partir de la config, las reglas del símbolo y el precio actual.
func Build(cfg *config.Config, r *binance.Rules, last float64) (*Plan, error) {
	p := &Plan{
		Symbol: cfg.Symbol, Side: cfg.Side, OrderType: cfg.OrderType, MarginType: cfg.MarginType,
		EntrySide: "BUY", ExitSide: "SELL", dir: 1,
		TPPct: cfg.TPPct, SLPct: cfg.SLPct, Margin: cfg.Margin, Leverage: cfg.Leverage, Rules: r,
	}
	if cfg.Side == "short" {
		p.EntrySide, p.ExitSide, p.dir = "SELL", "BUY", -1
	}
	p.Entry = last
	if cfg.OrderType == "limit" {
		p.Entry = r.RoundPrice(cfg.LimitPrice)
	}
	p.Qty = r.RoundQty(cfg.Margin * float64(cfg.Leverage) / p.Entry)
	if p.Qty < r.MinQty {
		return nil, fmt.Errorf("cantidad %.8f menor al mínimo %.8f; sube -margin o -leverage", p.Qty, r.MinQty)
	}
	if p.Qty*p.Entry < r.MinNotional {
		return nil, fmt.Errorf("notional %.2f menor al mínimo %.2f; sube -margin o -leverage",
			p.Qty*p.Entry, r.MinNotional)
	}
	p.Recalc(p.Entry)
	return p, nil
}

// Recalc recalcula TP/SL a partir de un precio de entrada (p. ej. el real de ejecución).
func (p *Plan) Recalc(entry float64) {
	p.TP = p.Rules.RoundPrice(entry * (1 + p.dir*p.TPPct/100))
	p.SL = p.Rules.RoundPrice(entry * (1 - p.dir*p.SLPct/100))
}
