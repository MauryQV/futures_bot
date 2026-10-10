
package config

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

func loadDotEnv(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k, v = strings.TrimSpace(k), strings.Trim(strings.TrimSpace(v), `"'`)
		if _, exists := os.LookupEnv(k); !exists {
			os.Setenv(k, v)
		}
	}
}

// Los errores de parseo de variables se acumulan: un TP_PCT=1,5 mal escrito
// debe fallar, no caer en silencio al valor por defecto.
var envErrs []string

func envStr(k, def string) string {
	if v := strings.TrimSpace(os.Getenv(k)); v != "" {
		return v
	}
	return def
}

func envFloat(k string, def float64) float64 {
	v := strings.TrimSpace(os.Getenv(k))
	if v == "" {
		return def
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		envErrs = append(envErrs, fmt.Sprintf("%s=%q no es un número", k, v))
		return def
	}
	return f
}

func envInt(k string, def int) int {
	v := strings.TrimSpace(os.Getenv(k))
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		envErrs = append(envErrs, fmt.Sprintf("%s=%q no es un entero", k, v))
		return def
	}
	return n
}

// Minutos de INTERVAL_MIN -> intervalo de velas de Binance.
var intervals = map[int]string{1: "1m", 3: "3m", 5: "5m", 15: "15m", 30: "30m", 60: "1h",
	120: "2h", 240: "4h", 360: "6h", 480: "8h", 720: "12h", 1440: "1d"}

type Config struct {
	Symbol     string
	Side       string // "long" | "short"
	OrderType  string // "market" | "limit"
	MarginType string // "ISOLATED" | "CROSSED"
	BaseURL    string
	APIKey     string
	APISecret  string

	Margin     float64
	Leverage   int
	TPPct      float64
	SLPct      float64
	LimitPrice float64
	Check      bool

	// Modo autónomo (solo demo)
	Auto            bool
	Strategy        string        // "llm" | "ema"
	IntervalMin     int           // minutos por vela = ciclo de decisión
	Poll            time.Duration // cada cuánto revisa la posición
	CooldownMin     int           // pausa tras cerrarse una posición
	MaxTrades       int           // máximo de entradas por día UTC
	MaxDailyLossPct float64       // % del balance diario; 0 = desactivado
	MinConfidence   int           // confianza mínima (0-100) para operar
	LevelBufferPct  float64       // distancia mínima a soporte/resistencia contrarios
	LogFile         string
	Fast, Slow      int // EMAs (strategy=ema)

	// LLM
	LLMProvider, LLMURL, LLMKey, LLMModel string
}

func (c *Config) Interval() string        { return intervals[c.IntervalMin] }
func (c *Config) Cooldown() time.Duration { return time.Duration(c.CooldownMin) * time.Minute }

func Load() (*Config, error) {
	loadDotEnv(".env")
	c := &Config{}
	flag.StringVar(&c.Symbol, "symbol", envStr("SYMBOL", "BTCUSDT"), "par de futuros USDT-M")
	flag.StringVar(&c.Side, "side", "long", "long | short (solo modo manual)")
	flag.Float64Var(&c.Margin, "margin", envFloat("MARGIN", 100), "margen en USDT (notional = margin * leverage)")
	flag.IntVar(&c.Leverage, "leverage", envInt("LEVERAGE", 5), "apalancamiento")
	flag.Float64Var(&c.TPPct, "tp", envFloat("TP_PCT", 2), "take profit en % de precio (0 = sin TP)")
	flag.Float64Var(&c.SLPct, "sl", envFloat("SL_PCT", 1), "stop loss en % de precio (0 = sin SL)")
	flag.StringVar(&c.OrderType, "type", "market", "market | limit (solo modo manual)")
	flag.Float64Var(&c.LimitPrice, "price", 0, "precio para ordenes limit")
	flag.StringVar(&c.MarginType, "margin-type", envStr("MARGIN_TYPE", "ISOLATED"), "ISOLATED | CROSSED")
	flag.StringVar(&c.BaseURL, "base-url", "https://demo-fapi.binance.com", "endpoint solo demo")
	flag.BoolVar(&c.Check, "check", false, "solo verificar mis claves")

	flag.BoolVar(&c.Auto, "auto", false, "modo autónomo: decide y opera solo (ignora -side/-type/-price)")
	flag.StringVar(&c.Strategy, "strategy", envStr("STRATEGY", "llm"), "llm | ema")
	flag.IntVar(&c.IntervalMin, "interval-min", envInt("INTERVAL_MIN", 30), "minutos por vela / ciclo de decisión")
	flag.DurationVar(&c.Poll, "poll", 10*time.Second, "frecuencia de revisión de posición en modo auto")
	flag.IntVar(&c.CooldownMin, "cooldown-min", envInt("COOLDOWN_MIN", 60), "pausa (min) tras cerrar una posición")
	flag.IntVar(&c.MaxTrades, "max-trades", envInt("MAX_TRADES_PER_DAY", 3), "máximo de operaciones por día (UTC)")
	flag.Float64Var(&c.MaxDailyLossPct, "max-daily-loss", envFloat("MAX_DAILY_LOSS_PCT", 5), "pérdida diaria máx. en % del balance (0 = off)")
	flag.IntVar(&c.MinConfidence, "min-confidence", envInt("MIN_CONFIDENCE", 70), "confianza mínima del decisor (0-100)")
	flag.Float64Var(&c.LevelBufferPct, "level-buffer", envFloat("LEVEL_BUFFER_PCT", 0.5), "no entrar si hay un nivel contrario a menos de este % (0 = off)")
	flag.StringVar(&c.LogFile, "log-file", envStr("LOG_FILE", ""), "archivo de log (además de la consola)")
	flag.IntVar(&c.Fast, "fast", 9, "EMA rápida (strategy=ema)")
	flag.IntVar(&c.Slow, "slow", 21, "EMA lenta (strategy=ema)")
	flag.Parse()

	c.APIKey = os.Getenv("BINANCE_API_KEY")
	c.APISecret = os.Getenv("BINANCE_API_SECRET")
	switch {
	case os.Getenv("GROQ_API_KEY") != "":
		c.LLMProvider, c.LLMKey = "groq", os.Getenv("GROQ_API_KEY")
		c.LLMURL = "https://api.groq.com/openai/v1/chat/completions"
		c.LLMModel = envStr("GROQ_MODEL", "openai/gpt-oss-120b")
	case os.Getenv("DEEPSEEK_API_KEY") != "":
		c.LLMProvider, c.LLMKey = "deepseek", os.Getenv("DEEPSEEK_API_KEY")
		c.LLMURL = "https://api.deepseek.com/chat/completions"
		c.LLMModel = envStr("DEEPSEEK_MODEL", "deepseek-chat")
	}

	c.LLMURL = envStr("LLM_URL", c.LLMURL) // opcional: endpoint alternativo/proxy
	c.Symbol = strings.ToUpper(c.Symbol)
	c.Side = strings.ToLower(c.Side)
	c.OrderType = strings.ToLower(c.OrderType)
	c.MarginType = strings.ToUpper(c.MarginType)
	c.Strategy = strings.ToLower(c.Strategy)
	c.BaseURL = strings.TrimRight(c.BaseURL, "/")
	if c.Auto {
		c.OrderType = "market" // en auto siempre market: el TP/SL se calcula sobre el precio real
	}
	return c, c.validate()
}

func (c *Config) validate() error {
	if len(envErrs) > 0 {
		return errors.New("variables de entorno inválidas: " + strings.Join(envErrs, "; "))
	}
	switch {
	case c.APIKey == "" || c.APISecret == "":
		return errors.New("define BINANCE_API_KEY y BINANCE_API_SECRET (claves de la cuenta DEMO)")
	case !strings.Contains(c.BaseURL, "demo") && !strings.Contains(c.BaseURL, "testnet"):
		return errors.New("por seguridad solo se permiten endpoints demo/testnet")
	case c.Side != "long" && c.Side != "short":
		return errors.New("-side debe ser long o short")
	case c.OrderType != "market" && c.OrderType != "limit":
		return errors.New("-type debe ser market o limit")
	case c.OrderType == "limit" && c.LimitPrice <= 0:
		return errors.New("con -type limit debes indicar -price")
	case c.MarginType != "ISOLATED" && c.MarginType != "CROSSED":
		return fmt.Errorf("-margin-type inválido: %s", c.MarginType)
	case c.Margin <= 0 || c.Leverage < 1:
		return errors.New("margin y leverage deben ser positivos")
	case c.TPPct < 0 || c.SLPct < 0:
		return errors.New("tp y sl no pueden ser negativos")
	}
	if !c.Auto {
		return nil
	}
	switch {
	case c.SLPct <= 0:
		return errors.New("en modo auto -sl es obligatorio (>0)")
	case c.Strategy != "llm" && c.Strategy != "ema":
		return errors.New("-strategy debe ser llm o ema")
	case c.Strategy == "llm" && c.LLMKey == "":
		return errors.New("strategy=llm requiere GROQ_API_KEY (o DEEPSEEK_API_KEY); o usa -strategy ema")
	case c.Interval() == "":
		return fmt.Errorf("INTERVAL_MIN=%d no es un intervalo de vela válido (1,3,5,15,30,60,120,240,360,480,720,1440)", c.IntervalMin)
	case c.Strategy == "ema" && (c.Fast < 2 || c.Fast >= c.Slow):
		return errors.New("-fast debe ser >= 2 y menor que -slow")
	case c.Poll < time.Second:
		return errors.New("-poll mínimo 1s")
	case c.MaxTrades < 1:
		return errors.New("MAX_TRADES_PER_DAY debe ser >= 1")
	case c.MinConfidence < 0 || c.MinConfidence > 100:
		return errors.New("MIN_CONFIDENCE debe estar entre 0 y 100")
	case c.MaxDailyLossPct < 0 || c.MaxDailyLossPct > 100:
		return errors.New("MAX_DAILY_LOSS_PCT debe estar entre 0 y 100")
	case c.LevelBufferPct < 0 || c.CooldownMin < 0:
		return errors.New("LEVEL_BUFFER_PCT y COOLDOWN_MIN no pueden ser negativos")
	}
	return nil
}
