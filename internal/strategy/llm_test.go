package strategy

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"futures_bot/internal/binance"
)

func TestParseDecision(t *testing.T) {
	ok := []struct {
		in, side string
		conf     int
	}{
		{`{"action":"long","confidence":82,"reason":"x"}`, "long", 82},
		{"```json\n{\"action\":\"SHORT\",\"confidence\":70.0,\"reason\":\"y\"}\n```", "short", 70},
		{"Aquí va: {\"action\":\"none\",\"confidence\":10,\"reason\":\"z\"} fin", "", 10},
	}
	for _, c := range ok {
		d, err := parseDecision(c.in)
		if err != nil || d.Side != c.side || d.Confidence != c.conf {
			t.Errorf("%q -> %+v, %v", c.in, d, err)
		}
	}
	bad := []string{``, `no json`, `{"action":"buy","confidence":90}`, `{"action":"long","confidence":150}`, `{"action":"long","confidence":-1}`, `{bad json}`}
	for _, in := range bad {
		if _, err := parseDecision(in); err == nil {
			t.Errorf("%q debía fallar", in)
		}
	}
}

func TestLLMDecideHTTP(t *testing.T) {
	var gotAuth, gotModel string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		b, _ := io.ReadAll(r.Body)
		var req map[string]any
		json.Unmarshal(b, &req)
		gotModel, _ = req["model"].(string)
		w.Write([]byte(`{"choices":[{"message":{"content":"{\"action\":\"long\",\"confidence\":75,\"reason\":\"ok\"}"}}]}`))
	}))
	defer srv.Close()

	l := NewLLM(srv.URL, "secret", "m1")
	var cs []binance.Candle
	for i := 0; i < 100; i++ {
		p := 100 + float64(i)
		cs = append(cs, binance.Candle{OpenTime: int64(i), Open: p, High: p + 1, Low: p - 1, Close: p, Volume: 5})
	}
	d, err := l.Decide(context.Background(), Snapshot{Symbol: "BTCUSDT", Interval: "30m", Price: 200, Candles: cs, TPPct: 1.5, SLPct: 1, Leverage: 3})
	if err != nil || d.Side != "long" || d.Confidence != 75 {
		t.Fatalf("%+v %v", d, err)
	}
	if gotAuth != "Bearer secret" || gotModel != "m1" {
		t.Fatalf("auth=%q model=%q", gotAuth, gotModel)
	}
}

func TestLLMHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(429)
		w.Write([]byte(`rate limited`))
	}))
	defer srv.Close()
	_, err := NewLLM(srv.URL, "k", "m").Decide(context.Background(), Snapshot{})
	if err == nil || !strings.Contains(err.Error(), "429") {
		t.Fatalf("esperaba error 429, obtuve %v", err)
	}
}

func TestLevelBlocked(t *testing.T) {
	// precio 100, resistencia 100.3 (0.3%) y soporte 99.7 (0.3%), buffer 0.5%
	if b, _ := LevelBlocked("long", 100, 99.7, 100.3, 0.5); !b {
		t.Error("long junto a resistencia debía bloquearse")
	}
	if b, _ := LevelBlocked("short", 100, 99.7, 100.3, 0.5); !b {
		t.Error("short junto a soporte debía bloquearse")
	}
	if b, _ := LevelBlocked("long", 100, 95, 105, 0.5); b {
		t.Error("long con espacio no debía bloquearse")
	}
	if b, _ := LevelBlocked("long", 110, 95, 105, 0.5); b {
		t.Error("ruptura por encima de la resistencia no debía bloquear el long")
	}
	if b, _ := LevelBlocked("long", 100, 99.7, 100.3, 0); b {
		t.Error("buffer 0 desactiva el filtro")
	}
}

func TestRSILevels(t *testing.T) {
	up := make([]float64, 30)
	for i := range up {
		up[i] = float64(i)
	}
	if RSI(up, 14) < 99 {
		t.Error("RSI de serie solo alcista debía ser ~100")
	}
	s, r := Levels([]binance.Candle{{Low: 5, High: 9}, {Low: 3, High: 8}, {Low: 4, High: 12}}, 2)
	if s != 3 || r != 12 {
		t.Fatalf("levels = %v, %v", s, r)
	}
}
