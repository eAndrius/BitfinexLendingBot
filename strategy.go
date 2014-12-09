// Copyright Andrius Sutas BitfinexLendingBot [at] motoko [dot] sutas [dot] eu

// TODO: evaluate different strategies:
// 	https://github.com/evdubs/Harmonia
//	https://github.com/mariodian/bitfinex-auto-lend
//	https://github.com/ah3dce/cascadebot

package main

import (
	"errors"
	"strings"
)

// StrategyConf ...
type StrategyConf struct {
	Active    string
	MarginBot MarginBotConf
}

func executeStrategy(conf BotConfig, dryRun bool) (err error) {
	// Sanity check
	if conf.API == nil {
		return errors.New("Please initialize the API instance first")
	}

	switch strings.ToLower(conf.Strategy.Active) {
	case "marginbot":
		return strategyMarginBot(conf, dryRun)
	}

	return errors.New("Undefined strategy")
}
