package binance

import (
	"encoding/json"
	"fmt"
	"net/url"

	"futures_bot/internal/numeric"
)

type Rules struct {
	Step, MinQty, Tick, MinNotional float64
	QtyDec, PriceDec                int
}

func (r *Rules) RoundQty(v float64) float64   { return numeric.FloorTo(v, r.Step) }
func (r *Rules) RoundPrice(v float64) float64 { return numeric.FloorTo(v, r.Tick) }
func (r *Rules) FmtQty(v float64) string      { return numeric.Format(v, r.QtyDec) }
func (r *Rules) FmtPrice(v float64) string    { return numeric.Format(v, r.PriceDec) }

func str(v any) string { s, _ := v.(string); return s }

func (c *Client) GetRules(symbol string) (*Rules, error) {
	b, err := c.do("GET", "/fapi/v1/exchangeInfo", nil, false)
	if err != nil {
		return nil, err
	}
	var info struct {
		Symbols []struct {
			Symbol  string           `json:"symbol"`
			Filters []map[string]any `json:"filters"`
		} `json:"symbols"`
	}
	if err := json.Unmarshal(b, &info); err != nil {
		return nil, err
	}
	for _, s := range info.Symbols {
		if s.Symbol != symbol {
			continue
		}
		r := &Rules{}
		for _, f := range s.Filters {
			switch str(f["filterType"]) {
			case "LOT_SIZE":
				r.Step, r.MinQty = numeric.Parse(str(f["stepSize"])), numeric.Parse(str(f["minQty"]))
				r.QtyDec = numeric.Decimals(str(f["stepSize"]))
			case "PRICE_FILTER":
				r.Tick = numeric.Parse(str(f["tickSize"]))
				r.PriceDec = numeric.Decimals(str(f["tickSize"]))
			case "MIN_NOTIONAL":
				r.MinNotional = numeric.Parse(str(f["notional"]))
			}
		}
		return r, nil
	}
	return nil, fmt.Errorf("símbolo %q no existe en futuros USDT-M", symbol)
}

func (c *Client) LastPrice(symbol string) (float64, error) {
	b, err := c.do("GET", "/fapi/v1/ticker/price", url.Values{"symbol": {symbol}}, false)
	if err != nil {
		return 0, err
	}
	var t struct {
		Price string `json:"price"`
	}
	if err := json.Unmarshal(b, &t); err != nil {
		return 0, err
	}
	return numeric.Parse(t.Price), nil
}
