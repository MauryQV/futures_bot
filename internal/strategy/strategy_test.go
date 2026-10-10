package strategy

import "testing"

func series(down, up int) []float64 {
	var v []float64
	p := 100.0
	for i := 0; i < 60; i++ { // lateral
		v = append(v, p)
	}
	for i := 0; i < down; i++ {
		p -= 1
		v = append(v, p)
	}
	for i := 0; i < up; i++ {
		p += 1
		v = append(v, p)
	}
	return v
}

// firstSignal recorre la serie vela a vela y devuelve la primera señal.
func firstSignal(s EMACross, v []float64, from int) string {
	for i := from; i <= len(v); i++ {
		if sig := s.Signal(v[:i]); sig != "" {
			return sig
		}
	}
	return ""
}

func TestCrossUp(t *testing.T) {
	s := EMACross{Fast: 3, Slow: 8}
	v := series(0, 20)
	if got := firstSignal(s, v, 61); got != "long" {
		t.Fatalf("esperaba long, obtuve %q", got)
	}
}

func TestCrossDown(t *testing.T) {
	s := EMACross{Fast: 3, Slow: 8}
	v := series(20, 0)
	if got := firstSignal(s, v, 61); got != "short" {
		t.Fatalf("esperaba short, obtuve %q", got)
	}
}

func TestNoSignalFlat(t *testing.T) {
	s := EMACross{Fast: 3, Slow: 8}
	if got := firstSignal(s, series(0, 0), 33); got != "" {
		t.Fatalf("mercado lateral no debe dar señal, obtuve %q", got)
	}
}

func TestNotEnoughData(t *testing.T) {
	if (EMACross{Fast: 3, Slow: 8}).Signal([]float64{1, 2, 3}) != "" {
		t.Fatal("con pocas velas no debe haber señal")
	}
}
