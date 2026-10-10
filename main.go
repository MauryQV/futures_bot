package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"

	"futures_bot/internal/binance"
	"futures_bot/internal/bot"
	"futures_bot/internal/config"
	"futures_bot/internal/plan"
	"futures_bot/internal/strategy"
	"futures_bot/internal/ui"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if cfg.LogFile != "" {
		f, err := os.OpenFile(cfg.LogFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			return fmt.Errorf("LOG_FILE: %w", err)
		}
		defer f.Close()
		bot.Out = io.MultiWriter(os.Stdout, f)
	}
	c := binance.New(cfg.BaseURL, cfg.APIKey, cfg.APISecret)
	if err := c.SyncTime(); err != nil {
		return err
	}
	if cfg.Check {
		return checkKeys(c)
	}
	rules, err := c.GetRules(cfg.Symbol)
	if err != nil {
		return err
	}
	if cfg.Auto {
		var dec strategy.Decider = strategy.EMACross{Fast: cfg.Fast, Slow: cfg.Slow}
		if cfg.Strategy == "llm" {
			dec = strategy.NewLLM(cfg.LLMURL, cfg.LLMKey, cfg.LLMModel)
		}
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
		defer stop()
		return bot.RunAuto(ctx, c, cfg, rules, dec)
	}
	last, err := c.LastPrice(cfg.Symbol)
	if err != nil {
		return err
	}
	p, err := plan.Build(cfg, rules, last)
	if err != nil {
		return err
	}

	ui.PrintPlan(p, c.BaseURL())
	if !ui.Confirm() {
		fmt.Println("Cancelado. No se envió ninguna orden.")
		return nil
	}
	return bot.Execute(c, p)
}

func checkKeys(c *binance.Client) error {
	bals, err := c.Balances()
	if err != nil {
		return err
	}
	fmt.Println("✔ Claves y permisos de Futures OK. Balances:")
	for _, b := range bals {
		if b.Balance != "0" && b.Balance != "0.00000000" {
			fmt.Printf("  %-6s balance=%s disponible=%s\n", b.Asset, b.Balance, b.AvailableBalance)
		}
	}
	return nil
}
