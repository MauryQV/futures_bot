package numeric

import (
	"math"
	"strconv"
	"strings"
)

func Decimals(s string) int {
	s = strings.TrimRight(s, "0")
	i := strings.Index(s, ".")
	if i < 0 {
		return 0
	}
	return len(s) - i - 1
}

func FloorTo(v, step float64) float64 { return math.Floor(v/step+1e-9) * step }

func Format(v float64, dec int) string { return strconv.FormatFloat(v, 'f', dec, 64) }

func Parse(s string) float64 { f, _ := strconv.ParseFloat(s, 64); return f }
