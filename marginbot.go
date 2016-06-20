// Copyright Andrius Sutas BitfinexLendingBot [at] motoko [dot] sutas [dot] eu
// Strategy inspired by: https://github.com/HFenter/MarginBot

package main

import (
	"errors"
	"log"
	"math"
	"strconv"
	"strings"

	"github.com/eAndrius/bitfinex-go"
)

// MarginBotConf ...
type MarginBotConf struct {
	MinDailyLendRate        float64
	SpreadLend              int
	GapBottom               float64
	GapTop                  float64
	ThirtyDayDailyThreshold float64
	HighHoldDailyRate       float64
	HighHoldAmount          float64
}

// MarginBotLoanOffer ...
type MarginBotLoanOffer struct {
	Amount, Rate float64
	Period       int
}

// MarginBotLoanOffers ...
type MarginBotLoanOffers []MarginBotLoanOffer

func strategyMarginBot(bconf BotConfig, dryRun bool) (err error) {
	api := bconf.API
	conf := bconf.Strategy.MarginBot
	activeWallet := strings.ToLower(bconf.Bitfinex.ActiveWallet)

	// Do sanity check: Is MinDailyLendRate set?
	if conf.MinDailyLendRate <= 0.003 { // 0.003% daily == 1.095% yearly
		log.Println("\tWARNING: minimum daily lend rate is low (" + strconv.FormatFloat(conf.MinDailyLendRate, 'f', -1, 64) + "%)")
	}

	// Do sanity check: Is HighHold rate higher than minimum daily rate?
	if conf.HighHoldDailyRate < conf.MinDailyLendRate { // 0.003% daily == 1.095% yearly
		log.Println("\tWARNING: HighHold daily lend rate (" +
			strconv.FormatFloat(conf.HighHoldDailyRate, 'f', -1, 64) +
			"% / day) is lower than MinDailyLendRate (" +
			strconv.FormatFloat(conf.MinDailyLendRate, 'f', -1, 64) + "% / day)")
	}

	// Cancel all active offers
	log.Println("\tCancelling all active " + activeWallet + " offers...")

	if !dryRun {
		err = api.CancelActiveOffersByCurrency(activeWallet)
		if err != nil {
			return
		}
	}

	// Update the lendbook
	log.Println("\tGetting current lendbook...")

	lendbook, err := api.Lendbook(activeWallet, 0, 10000)
	if err != nil {
		return
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
		available = math.Min(available, math.Min(available+bconf.Bitfinex.MaxActiveAmount-walletAmount, bconf.Bitfinex.MaxActiveAmount))

	}

	loanOffers := marginBotGetLoanOffers(available, minLoan, lendbook, conf)

	// Place the offers
	for _, o := range loanOffers {
		log.Println("\tPlacing offer: " +
			strconv.FormatFloat(o.Amount, 'f', -1, 64) + " " + activeWallet + " @ " +
			strconv.FormatFloat(o.Rate/365.0, 'f', -1, 64) + " % for " + strconv.Itoa(o.Period) + " days")

		if !dryRun {
			_, err = api.NewOffer(strings.ToUpper(activeWallet), o.Amount, o.Rate, o.Period, bitfinex.LEND)

			if err != nil {
				return errors.New("Failed to place new offer: " + err.Error())
			}
		}
	}

	log.Println("\tRun done.")

	return
}

func marginBotGetLoanOffers(fundsAvailable, minLoan float64, lendbook bitfinex.Lendbook, conf MarginBotConf) (loanOffers MarginBotLoanOffers) {
	// Sanity check: if it's less than minLonad we have nothing to do
	if fundsAvailable < minLoan {
		return
	}

	splitFundsAvailable := fundsAvailable

	// HighHold is a special case, substract from the available amount
	// HighHoldAmount = 0 => No HighHold required
	if conf.HighHoldAmount > minLoan {
		tmp := MarginBotLoanOffer{
			Amount: math.Min(fundsAvailable, conf.HighHoldAmount), // Make sure we have required balance to make HighHold offer
			Rate:   conf.HighHoldDailyRate * 365,
			Period: 30, // Always offer HighHold rate for 30 days
		}

		splitFundsAvailable -= tmp.Amount
		loanOffers = append(loanOffers, tmp)
	}

	// How many splits do we want?
	numSplits := conf.SpreadLend

	// is there anything left after the highhold?  if so, lets split it up
	if numSplits > 0 && splitFundsAvailable >= minLoan {

		// Round number to max precision supported by bitfinex
		amtEach := splitFundsAvailable / float64(numSplits)
		// Truncate to 8 decimal places
		amtEach = float64(int64(amtEach*100000000)) / 100000000.0

		// Minimize number of splits in case we cannot split in the number of required parts
		for amtEach <= minLoan {
			numSplits--
			amtEach = splitFundsAvailable / float64(numSplits)
			// Truncate to 8 decimal places
			amtEach = float64(int64(amtEach*100000000)) / 100000000.0

		}

		// Sanity check: is there any positive number of splits possible?
		if numSplits <= 0 {
			return
		}

		gapClimb := (conf.GapTop - conf.GapBottom) / float64(numSplits)
		nextLend := conf.GapBottom

		// Keep running total
		depthIndex := 0
		depthAmount := lendbook.Asks[depthIndex].Amount

		for numSplits > 0 {
			// Go trough lendbook until we meet our "nextLend" limit
			for depthAmount < nextLend && depthIndex < len(lendbook.Asks)-1 {
				depthIndex++
				depthAmount += lendbook.Asks[depthIndex].Amount
			}

			tmp := MarginBotLoanOffer{}
			tmp.Amount = amtEach

			// Make sure the gap setting rate is higher than the minimum lend rate...
			if lendbook.Asks[depthIndex].Rate < conf.MinDailyLendRate*365 {
				tmp.Rate = conf.MinDailyLendRate * 365
			} else {
				tmp.Rate = lendbook.Asks[depthIndex].Rate
			}

			// Are there loans that have high rate? If yes, lend them for as long as possible
			if conf.ThirtyDayDailyThreshold > 0 && lendbook.Asks[depthIndex].Rate >= conf.ThirtyDayDailyThreshold*365 {
				tmp.Period = 30
			} else {
				tmp.Period = 2
			}

			loanOffers = append(loanOffers, tmp)
			nextLend += gapClimb
			numSplits--
		}

	}

	return
}
