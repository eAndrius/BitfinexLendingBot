# BitfinexLendingBot Overview

BitfinexLendingBot (BLB) is a bot written in Go for automatic swap lending on the [Bitfinex](https://www.bitfinex.com/?refcode=7zVc3vSAbR) exchange. It works with all supported currencies (USD, BTC, LTC), is headless, does not require database setups and has unit testing for the lending strategies.

If you still don't have an account with [Bitfinex](https://www.bitfinex.com/?refcode=7zVc3vSAbR), please use referrer code **7zVc3vSAbR**, that way you will get a discount on your lending fees and will support the continued development of this project.

# Tutorial
0. Requirements

 * Go >= 1.2
 * [Bitfinex account](https://www.bitfinex.com/?refcode=7zVc3vSAbR)
 * (Optional) [glide](https://github.com/Masterminds/glide)
 * (Optional) Access to Crontab

1. Download or clone the main project and extract files to `BitfinexLendingBot` directory.

2. Download dependencies

    If using [glide](https://github.com/Masterminds/glide):

        cd BitfinexLendingBot/ && glide in && glide update

 Alternatively, with go get:

        cd BitfinexLendingBot/ && go get -u github.com/eAndrius/bitfinex-go

3. Compile bot (in `BitfinexLendingBot` directory)

        go build

4. Configure bot

    Generate [Bitfinex API key](https://www.bitfinex.com/account/api) and fill "APIKey" and "APISecret" fields in `default.conf`. For further options see [Configuration](#Configuration) section.

5. Run the bot and observe output. **Note:** no actual offers will be placed with `--dryrun` option.

        ./BitfinexLendingBot --updatelends --dryrun


## Flags

* `--conf` Select configuration file. **Default value:** "default.conf".

    Example:

        ./BitfinexLendingBot --conf=good_strategy.conf

* `--updatelends` Instruct Bot to update lend offerings based on the strategy in configuration file.

    Example:

        ./BitfinexLendingBot --updatelends

* `--dryrun` Output strategy decisions without placing actual lends on the exchange.

    Example:

        ./BitfinexLendingBot --updatelends --dryrun

* `--logtofile` Append Bot log to a file `blb.log` instead of stdout.

    Example:

        ./BitfinexLendingBot --updatelends --logtofile

## Scheduling

To run the Bot every 10 minutes with cron (`$ crontab -e`) use:

```
*/10 * * * * lockrun -n /tmp/blb.lock BitfinexLendingBot --updatelends --logtofile
```

Alternatively, to run in GNU Screen or similar use:

```bash
while [[ 1 ]]; do timeout 30s BitfinexLendingBot --updatelends --logtofile; sleep 10m; done
```

# Configuration

An example for multiple account configuration in `default.conf`:

```json
[
    {
        "bitfinex": {
            "APIKey": "<key1>",
            "APISecret": "<secret1>",
            "MinLoanUSD": 50,
            "ActiveWallet": "btc",
            "MaxActiveAmount": -1
        },

        "strategy": {
            "Active": "MarginBot",

            "MarginBot": {
                "MinDailyLendRate": 0.01,
                "SpreadLend": 3,
                "GapBottom": 100,
                "GapTop": 5000,
                "ThirtyDayDailyThreshold": 0.0,
                "HighHoldDailyRate": 0.05,
                "HighHoldAmount": 0.0
            }
        }
    },
    {
        "bitfinex": {
            "APIKey": "<key2>",
            "APISecret": "<secret2>",
            "MinLoanUSD": 50,
            "ActiveWallet": "btc",
            "MaxActiveAmount": -1
        },

        "strategy": {
            "Active": "CascadeBot",

            "CascadeBot": {
                "StartDailyLendRateFRRInc": 0.0012,
                "ReduceDailyLendRate": 0.0001,
                "MinDailyLendRate": 0.01,
                "ReductionIntervalMinutes": 10,
                "LendPeriod": 2,
                "ExponentialDecayMult": 1.0
            }
        }
    }
]
```

**Note:** Configuration file is a *list* of configurations, which means Bot will iterate over all acounts listed in the config file each time.

## Bitfinex

General settings for the Bitfinex exchange.

* `APIKey` String. Your generated Bitfinex API key.

* `APISecret` String. Your generated Bitfinex API key secret.

* `MinLoanUSD` Float. Minimum allowable loan on Bitfinex in USD.

* `ActiveWallet` String. Wallet to use for swap lending. **Values:** *usd, btc, ltc*.

* `MaxActiveAmount` Float. Maximum amount of currency to use for swap lending. **Values:** *<0 (negative)* - all available balance; *0 (zero)* - nothing (do not offer swaps); *>0 (positive)* - up to the amount specified.


## Strategy

Parameter for setting bot strategy for the account.

* `Active` String. Which strategy should the bot use for calculating swap lends. **Values:** *MarginBot, CascadeBot*.

### MarginBot Strategy

Lending strategy inspired by [MarginBot](https://github.com/HFenter/MarginBot).

* `MinDailyLendRate` Float. The lowest daily lend rate to use for any offer except the HighHold, as it is a special case (warning message is shown in case `HighHoldDailyRate` < `MinDailyLendRate`).

* `SpreadLend` Integer. The number of offers to split the available balance uniformly across the [`GapTop`, `GapBottom`] range. If set to *1* all balance will be offered at the rate of `GapBottom` position.

* `GapBottom` Float. The depth of lendbook (in volume) to move trough before placing the first offer. If set to *0* first offer will be placed at the rate of lowest ask.

* `GapTop` Float. The depth of lendbook (in volume) to move trough before placing the last offer. if `SpreadLend` is set to *>1* all offers will be distrbuted uniformly in the [`GapTop`, `GapBottom`] range.

* `ThirtyDayDailyThreshold` Float. Daily lend rate threshold after which we offer lends for 30 days as opposed to 2. If set to *0* all offers will be placed for a 2 day period.

* `HighHoldDailyRate` Float. Special High Hold offer for keeping a portion of wallet balance at a much higher daily rate. Does **not** count towards `SpreadLend` parameter. Always offered for 30 day period.

* `HighHoldAmount` Float. The amount of currency to offer at the `HighHoldDailyRate` rate. Does **not** count towards `SpreadLend` parameter. Always offered for 30 day period. If set to *0* High Hold offer is not made.

### CascadeBot Strategy

Lending strategy inspired by [CascadeBot](https://github.com/ah3dce/cascadebot). The strategy is modified so that starting daily lend rate is not defined as an absolute value, but rather than an increment (which can also be negative) to FRR.

* `StartDailyLendRateFRRInc` Float. The starting rate of FRR + `StartDailyLendRateFRRInc` that offers will be placed at.

* `ReduceDailyLendRate` Float. The rate at which to reduce already existing offers every `ReductionIntervalMinutes` minutes.

* `MinDailyLendRate` Float. The minimum daily lend rate that you're willing to lend at.

* `LendPeriod` Integer. The period for lend offers.

* `ReductionIntervalMinutes` Float. How often should the unlent offers` rate be decremented. Note that this parameter should be more than or equal to the interval at which bot is scheduled to run (usually 10 minutes).

* `ExponentialDecayMult` Float. Exponential decay constant which sets the decay rate. Set to *1* for a linear decay. Decay formula: ```NewDailyRate = (CurrentDailyRate - MinDailyLendRate) * ExponentialDecayMult + MinDailyLendRate```.

## Comparing Strategies

See a [weekly updated spreadsheet](https://docs.google.com/a/sutas.eu/spreadsheets/d/1lUwuN0KUwVIDBCxXOMNBsZyx_XsB1ND_KFmAJlUMRKQ) showing actual returns between different strategies and Flash Return Rate (Autorenew) Bitfinex option. For the bitcoin wallet balances start at 1 BTC for the each strategy and are always lent out in full (i.e. profits are accumulated). Strategy-default parameters are used.

# Licensing

Free for non-commercial (personal only) use. If you intend to use BitfinexLendingBot for a commercial purpose, please contact BitfinexLendingBot [at] motoko [dot] sutas [dot] eu to arrange a License.

# Like the Project? Show Support

฿ [1ASutaskUbCNiRxKcjwxA6PaymCZuqgLbL](bitcoin:17JKH8zRVM22SuYdYgfHJkgBQtUtYbRoJy?amount=0.01&label=Andrius%20Sutas&message=bitfinex-go)

![1ASutaskUbCNiRxKcjwxA6PaymCZuqgLbl](img/btc.png)
