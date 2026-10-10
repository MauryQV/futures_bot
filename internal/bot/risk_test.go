package bot

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"futures_bot/internal/binance"
)

func TestLossHit(t *testing.T) {
	if !lossHit(-50, 1000, 5) {
		t.Error("-50 sobre 1000 con límite 5% debe disparar")
	}
	if lossHit(-49, 1000, 5) || lossHit(30, 1000, 5) || lossHit(-10, 0, 5) {
		t.Error("no debía disparar")
	}
}

func TestDailyLossExceeded(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/fapi/v1/income":
			w.Write([]byte(`[{"incomeType":"REALIZED_PNL","income":"-40","asset":"USDT"},
				{"incomeType":"COMMISSION","income":"-5","asset":"USDT"},
				{"incomeType":"FUNDING_FEE","income":"-6","asset":"USDT"},
				{"incomeType":"TRANSFER","income":"500","asset":"USDT"}]`))
		case "/fapi/v2/balance":
			w.Write([]byte(`[{"asset":"USDT","balance":"949","availableBalance":"900"}]`))
		}
	}))
	defer srv.Close()
	hit, pnl, start, err := dailyLossExceeded(binance.New(srv.URL, "k", "s"), 5)
	if err != nil || !hit || pnl != -51 || start != 1000 {
		t.Fatalf("hit=%v pnl=%v start=%v err=%v", hit, pnl, start, err)
	}
	if hit, _, _, _ := dailyLossExceeded(binance.New(srv.URL, "k", "s"), 0); hit {
		t.Fatal("límite 0 debe desactivar")
	}
}
