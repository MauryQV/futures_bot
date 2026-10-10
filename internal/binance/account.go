package binance

import (
	"encoding/json"
	"net/url"
	"strconv"
	"time"

	"futures_bot/internal/numeric"
)

type Balance struct {
	Asset            string `json:"asset"`
	Balance          string `json:"balance"`
	AvailableBalance string `json:"availableBalance"`
}

func (c *Client) Balances() ([]Balance, error) {
	b, err := c.do("GET", "/fapi/v2/balance", nil, true)
	if err != nil {
		return nil, err
	}
	var out []Balance
	return out, json.Unmarshal(b, &out)
}

// USDTBalance devuelve el balance de la cartera USDT (sin PnL no realizado).
func (c *Client) USDTBalance() (float64, error) {
	bals, err := c.Balances()
	if err != nil {
		return 0, err
	}
	for _, b := range bals {
		if b.Asset == "USDT" {
			return numeric.Parse(b.Balance), nil
		}
	}
	return 0, nil
}

// NetPnLSince suma PnL realizado + comisiones + funding en USDT desde `since`.
func (c *Client) NetPnLSince(since time.Time) (float64, error) {
	b, err := c.do("GET", "/fapi/v1/income", url.Values{
		"startTime": {strconv.FormatInt(since.UnixMilli(), 10)}, "limit": {"1000"},
	}, true)
	if err != nil {
		return 0, err
	}
	var rows []struct {
		Type   string `json:"incomeType"`
		Income string `json:"income"`
		Asset  string `json:"asset"`
	}
	if err := json.Unmarshal(b, &rows); err != nil {
		return 0, err
	}
	var sum float64
	for _, r := range rows {
		if r.Asset != "USDT" {
			continue
		}
		switch r.Type {
		case "REALIZED_PNL", "COMMISSION", "FUNDING_FEE":
			sum += numeric.Parse(r.Income)
		}
	}
	return sum, nil
}
