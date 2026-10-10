// Package bot ejecuta un plan ya autorizado contra el exchange.
package bot

import (
	"errors"
	"fmt"

	"futures_bot/internal/binance"
	"futures_bot/internal/numeric"
	"futures_bot/internal/plan"
)

func Execute(c *binance.Client, p *plan.Plan) error {
	r := p.Rules
	if err := c.SetLeverage(p.Symbol, p.Leverage); err != nil {
		return fmt.Errorf("leverage: %w", err)
	}
	if err := c.SetMarginType(p.Symbol, p.MarginType); err != nil {
		return fmt.Errorf("margin type: %w", err)
	}

	req := binance.OrderRequest{Symbol: p.Symbol, Side: p.EntrySide, Qty: r.FmtQty(p.Qty)}
	if p.OrderType == "limit" {
		req.Price = r.FmtPrice(p.Entry)
	}
	res, err := c.NewOrder(req)
	if err != nil {
		return fmt.Errorf("orden de entrada: %w", err)
	}
	logf("✔ Orden de entrada #%d  estado=%s  avgPrice=%s", res.OrderID, res.Status, res.AvgPrice)

	// Con market, TP/SL se recalculan sobre el precio real de ejecución.
	if p.OrderType == "market" {
		if avg := numeric.Parse(res.AvgPrice); avg > 0 {
			p.Recalc(avg)
		}
	}

	conditional := func(kind string, trigger float64) bool {
		if err := c.NewConditional(p.Symbol, p.ExitSide, kind, r.FmtPrice(trigger)); err != nil {
			logf("✘ No se pudo colocar %s: %v", kind, err)
			return false
		}
		logf("✔ %s colocado en %s", kind, r.FmtPrice(trigger))
		return true
	}

	// El SL va primero: es la protección crítica.
	if p.SLPct > 0 && !conditional("STOP_MARKET", p.SL) {
		if p.OrderType == "market" {
			logf("⚠ SL no colocado: cerrando la posición para no quedar expuesto...")
			if cerr := closeNow(c, p); cerr != nil {
				return fmt.Errorf("SL falló Y el cierre de emergencia falló (REVISA LA POSICIÓN A MANO): %w", cerr)
			}
			return errors.New("SL falló; posición cerrada por seguridad")
		}
		logf("⚠ SL no colocado en orden limit: revisa a mano.")
	}
	if p.TPPct > 0 {
		conditional("TAKE_PROFIT_MARKET", p.TP)
	}
	if p.OrderType == "limit" {
		logf("ℹ Orden limit: revisa que el TP/SL siga siendo válido si la orden no llena.")
	}
	return nil
}

// closeNow cierra la posición del plan con una orden market reduceOnly.
func closeNow(c *binance.Client, p *plan.Plan) error {
	_, err := c.NewOrder(binance.OrderRequest{
		Symbol: p.Symbol, Side: p.ExitSide, Qty: p.Rules.FmtQty(p.Qty), ReduceOnly: true,
	})
	if err != nil {
		return err
	}
	logf("✔ Posición cerrada (reduceOnly market)")
	return nil
}
