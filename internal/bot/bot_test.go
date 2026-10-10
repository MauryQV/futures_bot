package bot

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"futures_bot/internal/binance"
	"futures_bot/internal/config"
	"futures_bot/internal/plan"
)

var rules = &binance.Rules{Step: 0.001, MinQty: 0.001, Tick: 0.1, MinNotional: 100, QtyDec: 3, PriceDec: 1}

func newPlan(t *testing.T) *plan.Plan {
	cfg := &config.Config{Symbol: "BTCUSDT", Side: "long", OrderType: "market", MarginType: "ISOLATED",
		Margin: 100, Leverage: 10, TPPct: 2, SLPct: 1}
	p, err := plan.Build(cfg, rules, 50000)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// Si el SL falla, debe enviarse un cierre market reduceOnly.
func TestSLFailureClosesPosition(t *testing.T) {
	var mu sync.Mutex
	var calls []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		calls = append(calls, r.URL.Path+"?"+r.URL.RawQuery)
		mu.Unlock()
		switch r.URL.Path {
		case "/fapi/v1/algoOrder":
			w.WriteHeader(400)
			w.Write([]byte(`{"code":-2021,"msg":"rejected"}`))
		case "/fapi/v1/order":
			w.Write([]byte(`{"orderId":1,"status":"FILLED","avgPrice":"50000.0"}`))
		default:
			w.Write([]byte(`{}`))
		}
	}))
	defer srv.Close()

	err := Execute(binance.New(srv.URL, "k", "s"), newPlan(t))
	if err == nil || !strings.Contains(err.Error(), "cerrada") {
		t.Fatalf("esperaba error de posición cerrada, obtuve %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	last := calls[len(calls)-1]
	if !strings.HasPrefix(last, "/fapi/v1/order") || !strings.Contains(last, "reduceOnly=true") || !strings.Contains(last, "side=SELL") {
		t.Fatalf("el último llamado debía ser el cierre reduceOnly, fue: %s", last)
	}
}

func TestHappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/fapi/v1/order" {
			w.Write([]byte(`{"orderId":1,"status":"FILLED","avgPrice":"50000.0"}`))
			return
		}
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()
	if err := Execute(binance.New(srv.URL, "k", "s"), newPlan(t)); err != nil {
		t.Fatal(err)
	}
}
