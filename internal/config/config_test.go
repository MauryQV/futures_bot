package config

import (
	"os"
	"strings"
	"testing"
)

func TestEnvDefaultsAndErrors(t *testing.T) {
	envErrs = nil
	os.Setenv("TP_PCT", "1.5")
	os.Setenv("LEVERAGE", "3")
	defer os.Unsetenv("TP_PCT")
	defer os.Unsetenv("LEVERAGE")
	if envFloat("TP_PCT", 2) != 1.5 || envInt("LEVERAGE", 5) != 3 || envInt("NOPE_X", 7) != 7 {
		t.Fatal("lectura de env incorrecta")
	}
	os.Setenv("SL_PCT", "1,5")
	defer os.Unsetenv("SL_PCT")
	envFloat("SL_PCT", 1)
	c := &Config{}
	if err := c.validate(); err == nil || !strings.Contains(err.Error(), "SL_PCT") {
		t.Fatalf("un SL_PCT mal escrito debe dar error, obtuve %v", err)
	}
	envErrs = nil
}

func TestIntervalMap(t *testing.T) {
	if (&Config{IntervalMin: 30}).Interval() != "30m" || (&Config{IntervalMin: 60}).Interval() != "1h" || (&Config{IntervalMin: 7}).Interval() != "" {
		t.Fatal("mapeo de INTERVAL_MIN incorrecto")
	}
}
