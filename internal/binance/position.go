package binance

import (
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"strconv"

	"futures_bot/internal/numeric"
)

// Candle es una vela cerrada.
type Candle struct {
	OpenTime                       int64
	Open, High, Low, Close, Volume float64
}

// ClosedCandles devuelve las últimas `limit` velas ya CERRADAS (descarta la que
// sigue en formación).
func (c *Client) ClosedCandles(symbol, interval string, limit int) ([]Candle, error) {
	b, err := c.do("GET", "/fapi/v1/klines", url.Values{
		"symbol": {symbol}, "interval": {interval}, "limit": {strconv.Itoa(limit + 1)},
	}, false)
	if err != nil {
		return nil, err
	}
	var raw [][]any
	if err := json.Unmarshal(b, &raw); err != nil {
		return nil, err
	}
	if len(raw) < 2 {
		return nil, fmt.Errorf("klines: pocas velas (%d)", len(raw))
	}
	raw = raw[:len(raw)-1]
	out := make([]Candle, 0, len(raw))
	for _, k := range raw {
		if len(k) < 6 {
			continue
		}
		ot, _ := k[0].(float64)
		out = append(out, Candle{
			OpenTime: int64(ot),
			Open:     numeric.Parse(str(k[1])), High: numeric.Parse(str(k[2])),
			Low: numeric.Parse(str(k[3])), Close: numeric.Parse(str(k[4])),
			Volume: numeric.Parse(str(k[5])),
		})
	}
	return out, nil
}

// PositionSize devuelve el tamaño absoluto de la posición abierta (0 = plano).
// Suma ambos lados por si la cuenta está en modo hedge.
func (c *Client) PositionSize(symbol string) (float64, error) {
	b, err := c.do("GET", "/fapi/v2/positionRisk", url.Values{"symbol": {symbol}}, true)
	if err != nil {
		return 0, err
	}
	var ps []struct {
		Amt string `json:"positionAmt"`
	}
	if err := json.Unmarshal(b, &ps); err != nil {
		return 0, err
	}
	var total float64
	for _, p := range ps {
		total += math.Abs(numeric.Parse(p.Amt))
	}
	return total, nil
}

// CancelAll cancela órdenes normales y condicionales (algo) pendientes del símbolo.
// Se llama cuando ya no hay posición para que un TP/SL huérfano no afecte a la
// siguiente operación.
func (c *Client) CancelAll(symbol string) error {
	p := func() url.Values { return url.Values{"symbol": {symbol}} }
	_, err1 := c.do("DELETE", "/fapi/v1/allOpenOrders", p(), true)
	_, err2 := c.do("DELETE", "/fapi/v1/algoOpenOrders", p(), true)
	if err1 != nil {
		return fmt.Errorf("allOpenOrders: %w", err1)
	}
	if err2 != nil {
		return fmt.Errorf("algoOpenOrders: %w", err2)
	}
	return nil
}
