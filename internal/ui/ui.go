// Package ui contiene la interacción por consola.
package ui

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"futures_bot/internal/plan"
)

func PrintPlan(p *plan.Plan, endpoint string) {
	r := p.Rules
	fmt.Println("════════════ PLAN DE OPERACIÓN (DEMO) ════════════")
	fmt.Printf("Endpoint     : %s\n", endpoint)
	fmt.Printf("Par          : %s   Dirección: %s (%s)\n", p.Symbol, strings.ToUpper(p.Side), p.EntrySide)
	fmt.Printf("Tipo orden   : %s   Precio ref.: %s\n", strings.ToUpper(p.OrderType), r.FmtPrice(p.Entry))
	fmt.Printf("Cantidad     : %s   Notional ≈ %.2f USDT\n", r.FmtQty(p.Qty), p.Qty*p.Entry)
	fmt.Printf("Margen       : %.2f USDT x%d (%s)\n", p.Margin, p.Leverage, p.MarginType)
	if p.TPPct > 0 {
		fmt.Printf("Take profit  : %s (%.2f%%)\n", r.FmtPrice(p.TP), p.TPPct)
	}
	if p.SLPct > 0 {
		fmt.Printf("Stop loss    : %s (%.2f%%)\n", r.FmtPrice(p.SL), p.SLPct)
	}
	fmt.Println("═══════════════════════════════════════════════════")
}

func Confirm() bool {
	fmt.Print("Escribe SI para ejecutar (cualquier otra cosa cancela): ")
	ans, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	return strings.ToUpper(strings.TrimSpace(ans)) == "SI"
}
