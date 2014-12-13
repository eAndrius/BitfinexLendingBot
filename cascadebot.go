// Copyright Andrius Sutas BitfinexLendingBot [at] motoko [dot] sutas [dot] eu
// Strategy inspired by: https://github.com/ah3dce/cascadebot
// Unlike the original cascadebot strategy, we start from FRR + Increment rate,
// so that the start lending rate settings would adapt dynamically to the lendbook
// and prevent offers from sitting unlent for long.

package main

import (
	"errors"
	"log"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/eAndrius/bitfinex-go"
)

// CascadeBotConf ...
type CascadeBotConf struct {
	StartDailyLendRateFRRInc float64
	MinDailyLendRate         float64
	ReductionIntervalMinutes float64
	ReduceDailyLendRate      float64
	ExponentialDecayMult     float64
	LendPeriod               int
}

const (
	cancel = iota
	lend
)

// CascadeBotAction ...
type CascadeBotAction struct {
	Action             int
	OfferID            int
	Amount, YearlyRate float64
	Period             int
}

// CascadeBotActions ...
type CascadeBotActions []CascadeBotAction

func strategyCascadeBot(bconf BotConfig, dryRun bool) (err error) {
	api := bconf.API
	conf := bconf.Strategy.CascadeBot
	activeWallet := strings.ToLower(bconf.Bitfinex.ActiveWallet)

	// Do sanity check: Is MinDailyLendRate set?
	if conf.MinDailyLendRate <= 0.003 { // 0.003% daily == 1.095% yearly
		log.Println("\tWARNING: minimum daily lend rate is low (" + strconv.FormatFloat(conf.MinDailyLendRate, 'f', -1, 64) + "%)")
	}

	// Get all active offers
	log.Println("\tGetting all active offers...")
	allOffers, err := api.ActiveOffers()
	if err != nil {
		return
	}

	// Filter only relevant offers
	log.Println("\tKeeping only " + activeWallet + " lend offers...")
	var offers bitfinex.Offers
	for _, o := range allOffers {
		if strings.ToLower(o.Currency) == activeWallet && strings.ToLower(o.Direction) == "lend" {
			offers = append(offers, o)
		}
	}

	log.Println("\tGetting current lendbook for FRR...")

	lendbook, err := api.Lendbook(activeWallet, 0, 10000)
	if err != nil {
		return
	}

	FRR := 1.0
	for _, o := range lendbook.Asks {
		if o.FRR {
			FRR = o.Rate / 365
			break
		}
	}

	// Sanity check: is the daily lend rate sane?
	if FRR+conf.StartDailyLendRateFRRInc >= 0.5 {
		log.Println("\tWARNING: Starting daily lend rate (" +
			strconv.FormatFloat(FRR+conf.StartDailyLendRateFRRInc, 'f', -1, 64) + " %/day) is unusually high")
	}

	log.Println("\tGetting current wallet balance...")
	balance, err := api.WalletBalances()
	if err != nil {
		return errors.New("Failed to get wallet funds: " + err.Error())
	}

	// Calculate minimum loan size
	minLoan := bconf.Bitfinex.MinLoanUSD
	if activeWallet != "usd" {
		log.Println("\tGetting current " + activeWallet + " ticker...")

		ticker, err := api.Ticker(activeWallet + "usd")
		if err != nil {
			return errors.New("Failed to get ticker: " + err.Error())
		}

		minLoan = bconf.Bitfinex.MinLoanUSD / ticker.Mid
	}

	// Sanity check: is there anything to lend?
	walletAmount := balance[bitfinex.WalletKey{"deposit", activeWallet}].Amount
	if walletAmount < minLoan {
		log.Println("\tWARNING: Wallet amount (" +
			strconv.FormatFloat(walletAmount, 'f', -1, 64) + " " + activeWallet + ") is less than the allowed minimum (" +
			strconv.FormatFloat(minLoan, 'f', -1, 64) + " " + activeWallet + ")")
	}

	// Determine available funds for trading
	available := balance[bitfinex.WalletKey{"deposit", activeWallet}].Available

	// Check if we need to limit our usage
	if bconf.Bitfinex.MaxActiveAmount >= 0 {
		available = math.Min(available, bconf.Bitfinex.MaxActiveAmount)
	}

	actions := cascadeBotGetActions(available, minLoan, FRR, offers, conf)

	// Execute the actions
	for _, a := range actions {
		if a.Action == cancel {
			log.Println("\tCanceling offer ID: " + strconv.Itoa(a.OfferID))

			if !dryRun {
				err = api.CancelOffer(a.OfferID)

				if err != nil {
					return errors.New("Failed to cancel offer: " + err.Error())
				}

			}
		} else if a.Action == lend {
			log.Println("\tPlacing offer: " +
				strconv.FormatFloat(a.Amount, 'f', -1, 64) + " " + activeWallet + " @ " +
				strconv.FormatFloat(a.YearlyRate/365, 'f', -1, 64) + " %/day for " + strconv.Itoa(a.Period) + " days")

			if !dryRun {
				_, err = api.NewOffer(strings.ToUpper(activeWallet), a.Amount, a.YearlyRate, a.Period, bitfinex.LEND)

				if err != nil {
					return errors.New("Failed to place new offer: " + err.Error())
				}
			}
		}

	}

	log.Println("\tRun done.")

	return
}

func cascadeBotGetActions(fundsAvailable, minLoan, dailyFRR float64, activeOffers bitfinex.Offers, conf CascadeBotConf) (actions CascadeBotActions) {
	// Update lend rates where needed
	for _, o := range activeOffers {
		// Check if we need to update the offer based on its timestamp
		offerDurationMinutes := (time.Now().Unix() - int64(o.Timestamp)) / 60
		if offerDurationMinutes >= int64(conf.ReductionIntervalMinutes) {
			// Cancel the offer first
			actions = append(actions,
				CascadeBotAction{Action: cancel, OfferID: o.ID})

			// Check if there is enough amount remaining so that we can re-lend it,
			// otherwise the offer's amount will just go back to the wallet
			// and be lent at the "starting" daily rate
			if o.RemainingAmount >= minLoan {
				// Adjust rate only one step
				// (e.g. to prevent offer going immediately to a minimum rate in the event of connection failure)
				newDailyRate := o.Rate / 365

				// Linear reduction
				newDailyRate -= conf.ReduceDailyLendRate

				// Exponential reduction
				newDailyRate = (newDailyRate-conf.MinDailyLendRate)*conf.ExponentialDecayMult + conf.MinDailyLendRate

				// Force minimum rate in case of wrong exponential decay user parameters
				newRate := math.Max(newDailyRate, conf.MinDailyLendRate) * 365

				// Make new offer at a different rate
				actions = append(actions, CascadeBotAction{Action: lend,
					YearlyRate: newRate, Amount: o.RemainingAmount, Period: o.Period})
			} else {
				fundsAvailable += o.RemainingAmount
			}
		}
	}

	// Are there spare funds to offer at the "starting" daily amount?
	if fundsAvailable >= minLoan {
		actions = append(actions, CascadeBotAction{Action: lend,
			YearlyRate: (dailyFRR + conf.StartDailyLendRateFRRInc) * 365, Amount: fundsAvailable, Period: 2})
	}

	return
}
