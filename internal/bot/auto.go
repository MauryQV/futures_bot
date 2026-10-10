package bot

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"futures_bot/internal/binance"
	"futures_bot/internal/config"
	"futures_bot/internal/plan"
	"futures_bot/internal/strategy"
)

// Out es el destino de los logs (consola, o consola + archivo si hay LOG_FILE).
var Out io.Writer = os.Stdout

func logf(format string, a ...any) {
	fmt.Fprintf(Out, "[%s] %s\n", time.Now().Format("2006-01-02 15:04:05"), fmt.Sprintf(format, a...))
}

// dailyLossExceeded dice si el resultado neto del día (UTC) alcanzó el límite.
// Balance de inicio de día = balance actual - PnL neto de hoy.
func dailyLossExceeded(c *binance.Client, limitPct float64) (bool, float64, float64, error) {
	if limitPct <= 0 {
		return false, 0, 0, nil
	}
	now := time.Now().UTC()
	midnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	pnl, err := c.NetPnLSince(midnight)
	if err != nil {
		return false, 0, 0, err
	}
	bal, err := c.USDTBalance()
	if err != nil {
		return false, 0, 0, err
	}
	return lossHit(pnl, bal-pnl, limitPct), pnl, bal - pnl, nil
}

func lossHit(pnl, startBal, limitPct float64) bool {
	return startBal > 0 && pnl < 0 && -pnl >= startBal*limitPct/100
}

// RunAuto opera solo: cada `Poll` revisa la posición y, si está plana, fuera de
// cooldown y dentro de los límites, consulta al decisor UNA vez por vela cerrada.
// Termina limpiamente con ctx (Ctrl+C); no cierra posiciones abiertas: quedan
// protegidas por su TP/SL en el exchange.
func RunAuto(ctx context.Context, c *binance.Client, cfg *config.Config, rules *binance.Rules, dec strategy.Decider) error {
	var (
		hadPosition   bool
		cooldownUntil time.Time
		lastCandle    int64
		trades        int
		day           = time.Now().UTC().Format("2006-01-02")
		limitLogged   bool
		lossLogged    bool
		lastSync      = time.Now()
		errStreak     int
	)

	// Al arrancar, si estamos planos limpiamos órdenes huérfanas de ejecuciones previas.
	if size, err := c.PositionSize(cfg.Symbol); err == nil && size == 0 {
		if err := c.CancelAll(cfg.Symbol); err != nil {
			logf("aviso al limpiar órdenes iniciales: %v", err)
		}
	} else if err == nil {
		hadPosition = true
		logf("ya hay una posición abierta (%.8f); espero a que cierre", size)
	}

	tick := func() error {
		if time.Since(lastSync) > 30*time.Minute {
			if err := c.SyncTime(); err == nil {
				lastSync = time.Now()
			}
		}
		if d := time.Now().UTC().Format("2006-01-02"); d != day {
			day, trades, limitLogged, lossLogged = d, 0, false, false
		}

		size, err := c.PositionSize(cfg.Symbol)
		if err != nil {
			return fmt.Errorf("positionRisk: %w", err)
		}
		if size > 0 {
			hadPosition = true
			return nil
		}
		if hadPosition { // acaba de cerrarse (TP, SL o manual)
			hadPosition = false
			if err := c.CancelAll(cfg.Symbol); err != nil {
				logf("aviso al cancelar órdenes pendientes: %v", err)
			}
			cooldownUntil = time.Now().Add(cfg.Cooldown())
			logf("posición cerrada; cooldown hasta %s", cooldownUntil.Format("15:04:05"))
			return nil
		}
		if time.Now().Before(cooldownUntil) {
			return nil
		}
		if trades >= cfg.MaxTrades {
			if !limitLogged {
				logf("límite diario alcanzado (%d operaciones); sin nuevas entradas hasta mañana UTC", trades)
				limitLogged = true
			}
			return nil
		}

		n := dec.Need() + 10
		if n < 100 {
			n = 100
		}
		candles, err := c.ClosedCandles(cfg.Symbol, cfg.Interval(), n)
		if err != nil {
			return fmt.Errorf("velas: %w", err)
		}
		last := candles[len(candles)-1]
		if last.OpenTime == lastCandle { // esta vela ya se evaluó
			return nil
		}
		lastCandle = last.OpenTime // se marca ya: si algo falla, no se reintenta en bucle
		vela := time.UnixMilli(last.OpenTime).Format("15:04")

		hit, pnl, startBal, err := dailyLossExceeded(c, cfg.MaxDailyLossPct)
		if err != nil {
			return fmt.Errorf("pnl diario: %w", err)
		}
		if hit {
			if !lossLogged {
				logf("🛑 pérdida diaria %.2f USDT sobre %.2f (límite %.1f%%): sin entradas hasta mañana UTC",
					pnl, startBal, cfg.MaxDailyLossPct)
				lossLogged = true
			}
			return nil
		}

		price, err := c.LastPrice(cfg.Symbol)
		if err != nil {
			return fmt.Errorf("precio: %w", err)
		}
		lb := 48
		sup, res := strategy.Levels(candles, lb)
		d, err := dec.Decide(ctx, strategy.Snapshot{
			Symbol: cfg.Symbol, Interval: cfg.Interval(), Price: price, Candles: candles,
			Support: sup, Resistance: res, TPPct: cfg.TPPct, SLPct: cfg.SLPct, Leverage: cfg.Leverage,
		})
		if err != nil {
			return fmt.Errorf("decisor: %w", err)
		}
		if d.Side == "" {
			logf("vela %s | %.2f | sin entrada (conf %d): %s", vela, price, d.Confidence, d.Reason)
			return nil
		}
		if d.Confidence < cfg.MinConfidence {
			logf("vela %s | propone %s pero confianza %d < %d: descartado (%s)", vela, d.Side, d.Confidence, cfg.MinConfidence, d.Reason)
			return nil
		}
		if blocked, why := strategy.LevelBlocked(d.Side, price, sup, res, cfg.LevelBufferPct); blocked {
			logf("vela %s | propone %s (conf %d) pero bloqueado: %s", vela, d.Side, d.Confidence, why)
			return nil
		}

		cc := *cfg
		cc.Side, cc.OrderType = d.Side, "market"
		p, err := plan.Build(&cc, rules, price)
		if err != nil {
			return fmt.Errorf("plan: %w", err)
		}
		logf("SEÑAL %s %s @ ~%s qty=%s TP=%s SL=%s conf=%d | %s", d.Side, p.Symbol,
			rules.FmtPrice(price), rules.FmtQty(p.Qty), rules.FmtPrice(p.TP), rules.FmtPrice(p.SL), d.Confidence, d.Reason)

		trades++ // cuenta el intento aunque falle
		if err := Execute(c, p); err != nil {
			return fmt.Errorf("ejecución: %w", err)
		}
		hadPosition = true
		return nil
	}

	src := cfg.Strategy
	if src == "llm" {
		src = fmt.Sprintf("llm/%s/%s", cfg.LLMProvider, cfg.LLMModel)
	}
	logf("modo AUTO en %s | %s | %s | velas %s | TP %.2f%% SL %.2f%% | x%d %s margen %.2f",
		c.BaseURL(), cfg.Symbol, src, cfg.Interval(), cfg.TPPct, cfg.SLPct, cfg.Leverage, cfg.MarginType, cfg.Margin)
	logf("límites: %d ops/día | pérdida diaria %.1f%% | confianza >= %d | buffer niveles %.2f%% | cooldown %dm",
		cfg.MaxTrades, cfg.MaxDailyLossPct, cfg.MinConfidence, cfg.LevelBufferPct, cfg.CooldownMin)
	logf("Ctrl+C para detener (las posiciones abiertas conservan su TP/SL)")

	timer := time.NewTimer(0)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			logf("detenido")
			return nil
		case <-timer.C:
		}
		if err := tick(); err != nil {
			errStreak++
			logf("error (%d seguidos): %v", errStreak, err)
			if errStreak >= 10 { // kill switch: algo anda mal de forma persistente
				return fmt.Errorf("demasiados errores seguidos, me detengo: %w", err)
			}
		} else {
			errStreak = 0
		}
		timer.Reset(cfg.Poll)
	}
}
