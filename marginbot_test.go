// Copyright Andrius Sutas BitfinexLendingBot [at] motoko [dot] sutas [dot] eu

package main

import (
	"math"
	"strconv"
	"testing"

	"github.com/eAndrius/bitfinex-go"
)

func TestMarginBotGetLoanOffers_MinDailyLendRate(t *testing.T) {
	conf := MarginBotConf{
		MinDailyLendRate: 1, // 365 APR
		SpreadLend:       1, // Generate only one offer
		GapBottom:        0, // Start from the lowest offer in the lendbook
	}

	// All offers in landbook are below our minimum of "MinDailyLendRate"
	lendbook := bitfinex.Lendbook{
		Asks: []bitfinex.LendbookOffer{
			bitfinex.LendbookOffer{Rate: 0.1 * 365},
			bitfinex.LendbookOffer{Rate: 0.2 * 365},
			bitfinex.LendbookOffer{Rate: 0.3 * 365},
		},
	}

	// Balance 100, no min for offers
	loanOffers := marginBotGetLoanOffers(100, 0, lendbook, conf)

	// Check if only one offer was returned
	if len(loanOffers) != 1 {
		t.Error("Returned wrong number of loan offers (" + strconv.Itoa(len(loanOffers)) + ", expected: 1)")
	}

	// Check for expected rate
	if math.Abs(loanOffers[0].Rate-1*365) > 0.0000000001 {
		t.Error("Returned wrong minimum offer rate (" + strconv.FormatFloat(loanOffers[0].Rate, 'f', -1, 64) + " APR, expected: 365 APR)")
	}

	// Check for expected period
	if loanOffers[0].Period != 2 {
		t.Error("Returned wrong offer period (" + strconv.Itoa(loanOffers[0].Period) + " days, expected: 2 days)")
	}

	// Check for expected amount
	if math.Abs(loanOffers[0].Amount-100) > 0.0000000001 {
		t.Error("Returned wrong offer amount (" + strconv.FormatFloat(loanOffers[0].Amount, 'f', -1, 64) + " , expected: 100)")
	}

	// Check that offers are not placed if there are insufficient funds
	// Testing here because we are guaranteed to return at least one offer with the above settings
	// Available balance 100, 101 required minimum
	loanOffers = marginBotGetLoanOffers(100, 101, lendbook, conf)

	// Check if none offers were returned
	if len(loanOffers) != 0 {
		t.Error("Returned wrong number of loan offers (" + strconv.Itoa(len(loanOffers)) + ", expected: 0)")
	}
}

func TestMarginBotGetLoanOffers_ThirtyDayDailyThreshold(t *testing.T) {
	conf := MarginBotConf{
		MinDailyLendRate:        0.1, // 36.5% / year
		SpreadLend:              1,   // Generate only one offer
		GapBottom:               0,   // Start from the lowest offer in the lendbook
		ThirtyDayDailyThreshold: 1,   // if APR >= 365% / year => offer for 30 days
	}

	lendbook := bitfinex.Lendbook{
		Asks: []bitfinex.LendbookOffer{
			bitfinex.LendbookOffer{Rate: 1 * 365}, // APR == ThirtyDayDailyMin => make offer for 30 days
			bitfinex.LendbookOffer{Rate: 2 * 365},
			bitfinex.LendbookOffer{Rate: 3 * 365},
		},
	}

	// Balance 100, no min for offers
	loanOffers := marginBotGetLoanOffers(100, 0, lendbook, conf)

	// Check if only one offer was returned
	if len(loanOffers) != 1 {
		t.Error("Returned wrong number of loan offers (" + strconv.Itoa(len(loanOffers)) + ", expected: 1)")
	}

	// Check for expected rate
	if math.Abs(loanOffers[0].Rate-1*365) > 0.0000000001 {
		t.Error("Returned wrong minimum offer rate (" + strconv.FormatFloat(loanOffers[0].Rate, 'f', -1, 64) + " APR, expected: 365 APR)")
	}

	// Check for expected period
	if loanOffers[0].Period != 30 {
		t.Error("Returned wrong offer period (" + strconv.Itoa(loanOffers[0].Period) + " days, expected: 30 days)")
	}

	// Check for expected amount
	if math.Abs(loanOffers[0].Amount-100.0) > 0.0000000001 {
		t.Error("Returned wrong offer amount (" + strconv.FormatFloat(loanOffers[0].Amount, 'f', -1, 64) + ", expected: 100)")
	}
}

func TestMarginBotGetLoanOffers_HighHold(t *testing.T) {
	conf := MarginBotConf{
		HighHoldAmount:    10,
		HighHoldDailyRate: 1, // 365 APR
	}

	lendbook := bitfinex.Lendbook{}

	// Balance 100, no min for offers
	loanOffers := marginBotGetLoanOffers(100, 0, lendbook, conf)

	// Check if only one offer was returned
	if len(loanOffers) != 1 {
		t.Error("Returned wrong number of loan offers (" + strconv.Itoa(len(loanOffers)) + ", expected: 1)")
	}

	// Check for expected rate for HighHold
	if math.Abs(loanOffers[0].Rate-1*365) > 0.0000000001 {
		t.Error("Returned wrong minimum offer rate (" + strconv.FormatFloat(loanOffers[0].Rate, 'f', -1, 64) + " APR, expected: 365 APR)")
	}

	// Check for expected period for HighHold
	if loanOffers[0].Period != 30 {
		t.Error("Returned wrong offer period (" + strconv.Itoa(loanOffers[0].Period) + " days, expected: 30 days)")
	}

	// Check for expected amount for HighHold
	if math.Abs(loanOffers[0].Amount-10) > 0.0000000001 {
		t.Error("Returned wrong offer amount (" + strconv.FormatFloat(loanOffers[0].Amount, 'f', -1, 64) + " , expected: 10)")
	}

	// Balance only 5 (less than HighHold amount), no min for offers
	loanOffers = marginBotGetLoanOffers(5, 0, lendbook, conf)

	// Check for expected amount for HighHold
	if math.Abs(loanOffers[0].Amount-5) > 0.0000000001 {
		t.Error("Returned wrong offer amount (" + strconv.FormatFloat(loanOffers[0].Amount, 'f', -1, 64) + " , expected: 10)")
	}
}

func TestMarginBotGetLoanOffers_General(t *testing.T) {
	// Fill lendbook with asks for: 0.1, 0.2...5.0 % daily
	asks := []bitfinex.LendbookOffer{}
	for i := 1; i <= 50; i++ {
		asks = append(asks, bitfinex.LendbookOffer{Rate: (0.1 * float64(i)) * 365, Amount: 0.1})
	}
	lendbook := bitfinex.Lendbook{Asks: asks}

	conf := MarginBotConf{
		SpreadLend: 4,   // Split into 4 offers
		GapBottom:  3.2, // Skip the lowest 31 offers in the landbook
		GapTop:     4.3, // Make split accross 1 btc range

		MinDailyLendRate:        3.3, // One potential offer (3.2 % / day) in the lendbook is below minimum (3.3 % / day)
		ThirtyDayDailyThreshold: 4,   // One potential offer (4.1 % / day) in the lendbook is >= the 30 day threshold

		HighHoldDailyRate: 365, // Make special HighHold offer at 365 * 365 APR
		HighHoldAmount:    10,  // For 10 currency units
	}

	// Available balance 100, no minimum
	loanOffers := marginBotGetLoanOffers(110, 0, lendbook, conf)

	// Check if 5 offers were returned (4 from split + 1 from HighHold)
	if len(loanOffers) != 5 {
		t.Error("Returned wrong number of loan offers (" + strconv.Itoa(len(loanOffers)) + ", expected: 5)")
	}

	// Populate expected offers
	expectedOffers := MarginBotLoanOffers{
		MarginBotLoanOffer{Amount: 10, Rate: 365. * 365., Period: 30}, // Special HighHold offer
		MarginBotLoanOffer{Amount: 25, Rate: 4.1 * 365., Period: 30},  // Offer which has a rate abote the ThirtyDayDailyThreshold
		MarginBotLoanOffer{Amount: 25, Rate: 3.3 * 365., Period: 2},   // Offer which has a below minimum rate that was increased
		MarginBotLoanOffer{Amount: 25, Rate: 3.5 * 365., Period: 2},   // Normal offer
		MarginBotLoanOffer{Amount: 25, Rate: 3.8 * 365., Period: 2},   // Normal offer

	}

	// Check for expected offers (in any order)
	for _, eo := range expectedOffers {
		for i, lo := range loanOffers {
			if eo.Period == lo.Period && math.Abs(eo.Rate-lo.Rate) < 0.0000000001 && math.Abs(eo.Amount-lo.Amount) < 0.0000000001 {
				// Remove *only one* matching offer
				loanOffers = append(loanOffers[:i], loanOffers[i+1:]...)
				break
			}
		}
	}

	if len(loanOffers) != 0 {
		t.Errorf("Returned wrong loan offers (expected: %v)", expectedOffers)
	}
}
