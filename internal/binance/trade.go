package binance

import (
	"encoding/json"
	"net/url"
	"strconv"
)

// OrderRequest describe una orden de entrada. Price vacío = MARKET.
type OrderRequest struct {
	Symbol, Side, Qty, Price string
	ReduceOnly               bool
}

type OrderResult struct {
	OrderID  int64  `json:"orderId"`
	Status   string `json:"status"`
	AvgPrice string `json:"avgPrice"`
}

func (c *Client) SetLeverage(symbol string, lev int) error {
	_, err := c.do("POST", "/fapi/v1/leverage",
		url.Values{"symbol": {symbol}, "leverage": {strconv.Itoa(lev)}}, true)
	return err
}

// SetMarginType ignora el error -4046 (ya estaba en ese modo).
func (c *Client) SetMarginType(symbol, mt string) error {
	_, err := c.do("POST", "/fapi/v1/marginType",
		url.Values{"symbol": {symbol}, "marginType": {mt}}, true)
	if ae, ok := err.(*APIError); ok && ae.Code == -4046 {
		return nil
	}
	return err
}

func (c *Client) NewOrder(r OrderRequest) (*OrderResult, error) {
	p := url.Values{"symbol": {r.Symbol}, "side": {r.Side}, "quantity": {r.Qty},
		"newOrderRespType": {"RESULT"}}
	if r.ReduceOnly {
		p.Set("reduceOnly", "true")
	}
	if r.Price == "" {
		p.Set("type", "MARKET")
	} else {
		p.Set("type", "LIMIT")
		p.Set("price", r.Price)
		p.Set("timeInForce", "GTC")
	}
	b, err := c.do("POST", "/fapi/v1/order", p, true)
	if err != nil {
		return nil, err
	}
	var res OrderResult
	return &res, json.Unmarshal(b, &res)
}

// NewConditional coloca STOP_MARKET o TAKE_PROFIT_MARKET que cierra toda la
// posición, usando el endpoint Algo Order.
func (c *Client) NewConditional(symbol, side, kind, trigger string) error {
	_, err := c.do("POST", "/fapi/v1/algoOrder", url.Values{
		"algoType": {"CONDITIONAL"}, "symbol": {symbol}, "side": {side},
		"type": {kind}, "triggerPrice": {trigger},
		"closePosition": {"true"}, "workingType": {"MARK_PRICE"},
	}, true)
	return err
}
