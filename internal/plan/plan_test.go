package plan

import (
	"testing"

	"futures_bot/internal/binance"
	"futures_bot/internal/config"
)

var rules = &binance.Rules{Step: 0.001, MinQty: 0.001, Tick: 0.1, MinNotional: 100, QtyDec: 3, PriceDec: 1}

func TestLongLevels(t *testing.T) {
	cfg := &config.Config{Symbol: "BTCUSDT", Side: "long", OrderType: "market",
		Margin: 100, Leverage: 10, TPPct: 2, SLPct: 1}
	p, err := Build(cfg, rules, 50000)
	if err != nil {
		t.Fatal(err)
	}
	if p.EntrySide != "BUY" || p.ExitSide != "SELL" {
		t.Fatalf("lados incorrectos: %s/%s", p.EntrySide, p.ExitSide)
	}
	if p.Qty != 0.02 {
		t.Fatalf("qty = %v, quería 0.02", p.Qty)
	}
	if p.TP <= 50000 || p.SL >= 50000 {
		t.Fatalf("long: TP debe estar arriba y SL abajo (TP=%v SL=%v)", p.TP, p.SL)
	}
}

func TestShortLevels(t *testing.T) {
	cfg := &config.Config{Symbol: "BTCUSDT", Side: "short", OrderType: "market",
		Margin: 100, Leverage: 10, TPPct: 2, SLPct: 1}
	p, err := Build(cfg, rules, 50000)
	if err != nil {
		t.Fatal(err)
	}
	if p.EntrySide != "SELL" || p.ExitSide != "BUY" {
		t.Fatalf("lados incorrectos: %s/%s", p.EntrySide, p.ExitSide)
	}
	if p.TP >= 50000 || p.SL <= 50000 {
		t.Fatalf("short: TP debe estar abajo y SL arriba (TP=%v SL=%v)", p.TP, p.SL)
	}
}

func TestMinNotional(t *testing.T) {
	cfg := &config.Config{Side: "long", OrderType: "market", Margin: 5, Leverage: 1}
	if _, err := Build(cfg, rules, 50000); err == nil {
		t.Fatal("esperaba error por notional mínimo")
	}
}
