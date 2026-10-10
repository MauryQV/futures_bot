package strategy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// LLM consulta un endpoint tipo OpenAI /chat/completions (Groq, DeepSeek...).
type LLM struct {
	URL, Key, Model string
	HTTP            *http.Client
}

func NewLLM(url, key, model string) *LLM {
	return &LLM{URL: url, Key: key, Model: model, HTTP: &http.Client{Timeout: 90 * time.Second}}
}

func (l *LLM) Need() int { return 100 }

const systemPrompt = `You are a cautious crypto-futures trading analyst. You receive recent CLOSED candles and indicators for one symbol and must decide whether to open a position NOW.
Rules:
- "none" is a perfectly good answer and should be your default when the setup is unclear, choppy or the risk/reward is poor.
- If you choose a side, the position will be closed by a fixed take-profit (tp_pct) or stop-loss (sl_pct) measured on price.
- "confidence" is your estimated probability, in percent (0-100), that price hits the take-profit BEFORE the stop-loss.
- Respond with ONLY a JSON object, no markdown, no extra text:
{"action":"long"|"short"|"none","confidence":<integer 0-100>,"reason":"<max 200 chars>"}`

type promptData struct {
	Symbol     string      `json:"symbol"`
	Interval   string      `json:"interval"`
	Price      float64     `json:"price"`
	TPPct      float64     `json:"tp_pct"`
	SLPct      float64     `json:"sl_pct"`
	Leverage   int         `json:"leverage"`
	EMA9       float64     `json:"ema9"`
	EMA21      float64     `json:"ema21"`
	RSI14      float64     `json:"rsi14"`
	Support    float64     `json:"support"`
	Resistance float64     `json:"resistance"`
	Candles    [][]float64 `json:"candles_ohlcv_oldest_first"`
}

func buildPrompt(s Snapshot) (string, error) {
	closes := make([]float64, len(s.Candles))
	for i, k := range s.Candles {
		closes[i] = k.Close
	}
	last := s.Candles
	if len(last) > 40 {
		last = last[len(last)-40:]
	}
	d := promptData{Symbol: s.Symbol, Interval: s.Interval, Price: s.Price, TPPct: s.TPPct,
		SLPct: s.SLPct, Leverage: s.Leverage, Support: s.Support, Resistance: s.Resistance,
		RSI14: RSI(closes, 14)}
	if n := len(closes); n > 0 {
		d.EMA9, d.EMA21 = EMA(closes, 9)[n-1], EMA(closes, 21)[n-1]
	}
	for _, k := range last {
		d.Candles = append(d.Candles, []float64{k.Open, k.High, k.Low, k.Close, k.Volume})
	}
	b, err := json.Marshal(d)
	return string(b), err
}

func (l *LLM) Decide(ctx context.Context, s Snapshot) (Decision, error) {
	prompt, err := buildPrompt(s)
	if err != nil {
		return Decision{}, err
	}
	body, _ := json.Marshal(map[string]any{
		"model": l.Model,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": prompt},
		},
		"temperature":           0.2,
		"max_completion_tokens": 2048, // los modelos con razonamiento necesitan margen
	})
	req, err := http.NewRequestWithContext(ctx, "POST", l.URL, bytes.NewReader(body))
	if err != nil {
		return Decision{}, err
	}
	req.Header.Set("Authorization", "Bearer "+l.Key)
	req.Header.Set("Content-Type", "application/json")
	resp, err := l.HTTP.Do(req)
	if err != nil {
		return Decision{}, fmt.Errorf("llm: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		msg := string(raw)
		if len(msg) > 300 {
			msg = msg[:300]
		}
		return Decision{}, fmt.Errorf("llm HTTP %d: %s", resp.StatusCode, msg)
	}
	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(raw, &out); err != nil || len(out.Choices) == 0 {
		return Decision{}, errors.New("llm: respuesta sin choices")
	}
	return parseDecision(out.Choices[0].Message.Content)
}

// parseDecision extrae y valida el JSON del modelo (tolera ```json ... ``` y texto extra).
func parseDecision(content string) (Decision, error) {
	i, j := strings.Index(content, "{"), strings.LastIndex(content, "}")
	if i < 0 || j <= i {
		return Decision{}, fmt.Errorf("llm: sin JSON en la respuesta: %.120q", content)
	}
	var r struct {
		Action     string  `json:"action"`
		Confidence float64 `json:"confidence"`
		Reason     string  `json:"reason"`
	}
	if err := json.Unmarshal([]byte(content[i:j+1]), &r); err != nil {
		return Decision{}, fmt.Errorf("llm: JSON inválido: %w", err)
	}
	if r.Confidence < 0 || r.Confidence > 100 {
		return Decision{}, fmt.Errorf("llm: confianza fuera de rango (%v)", r.Confidence)
	}
	d := Decision{Confidence: int(r.Confidence), Reason: r.Reason}
	switch a := strings.ToLower(strings.TrimSpace(r.Action)); a {
	case "long", "short":
		d.Side = a
	case "none", "":
	default:
		return Decision{}, fmt.Errorf("llm: acción desconocida %q", r.Action)
	}
	return d, nil
}
