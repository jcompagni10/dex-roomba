package roomba

import (
	"fmt"
	"math"
	"math/rand"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/jcompagni10/dex-roomba/x/base"
	"github.com/jcompagni10/dex-roomba/x/dex"
	"github.com/jcompagni10/dex-roomba/x/oracle"
	math_utils "github.com/neutron-org/neutron/v6/utils/math"
	dextypes "github.com/neutron-org/neutron/v6/x/dex/types"
	"google.golang.org/grpc"
)

type Denom struct {
	Symbol    string
	IBCDenom  string
	Exponent  int64
	PriceFeed bool
}

var (
	DefaultSpread = math_utils.MustNewPrecDecFromStr("0.02")
	USDC          = Denom{Symbol: "USDC", IBCDenom: "ibc/B559A80D62249C8AA07A380E2A2BEA6E5CA9A6F079C912C3A9E9B494105E4F81", Exponent: 6, PriceFeed: true}
	dustDenoms    = []Denom{
		{Symbol: "NTRN", IBCDenom: "untrn", Exponent: 6, PriceFeed: true},
		{Symbol: "ATOM", IBCDenom: "ibc/C4CFF46FD6DE35CA4CF4CE031E643C8FDC9BA4B99AE598E9B0ED98FE3A2319F9", Exponent: 6, PriceFeed: true},
		{Symbol: "TIA", IBCDenom: "ibc/773B4D0A3CD667B2275D5A4A7A2F0909C0BA0F4059C0B9181E680DDF4965DCC7", Exponent: 6, PriceFeed: true},
		{Symbol: "BTC", IBCDenom: "ibc/DF8722298D192AAB85D86D0462E8166234A6A9A572DD4A2EA7996029DF4DB363", Exponent: 8, PriceFeed: true}, // wBTC.axl
		{Symbol: "DYDX", IBCDenom: "ibc/2CB87BCE0937B1D1DFCEE79BE4501AAF3C265E923509AEAC410AD85D27F35130", Exponent: 18, PriceFeed: true},
		{Symbol: "OSMO", IBCDenom: "ibc/376222D6D9DAE23092E29740E56B758580935A6D77C24C2ABD57A6A78A1F3955", Exponent: 6, PriceFeed: true},
		{Symbol: "ETH", IBCDenom: "ibc/A585C2D15DCD3B010849B453A2CFCB5E213208A5AB665691792684C26274304D", Exponent: 18, PriceFeed: true}, // wETH.axl
		{Symbol: "dATOM", IBCDenom: "factory/neutron1k6hr0f83e7un2wjf29cspk7j69jrnskk65k3ek2nj9dztrlzpj6q00rtsa/udatom", Exponent: 6, PriceFeed: false},
		{Symbol: "dTIA", IBCDenom: "factory/neutron1ut4c6pv4u6vyu97yw48y8g7mle0cat54848v6m97k977022lzxtsaqsgmq/udtia", Exponent: 6, PriceFeed: false},
		{Symbol: "dNTRN", IBCDenom: "factory/neutron1frc0p5czd9uaaymdkug2njz7dc7j65jxukp9apmt9260a8egujkspms2t2/udntrn", Exponent: 6, PriceFeed: false},
		{Symbol: "wstETH", IBCDenom: "factory/neutron1ug740qrkquxzrk2hh29qrlx3sktkfml3je7juusc2te7xmvsscns0n2wry/wstETH", Exponent: 18, PriceFeed: false},
		{Symbol: "ASTRO", IBCDenom: "factory/neutron1ffus553eet978k024lmssw0czsxwr97mggyv85lpcsdkft8v9ufsz3sa07/astro", Exponent: 6, PriceFeed: false},
		{Symbol: "BTC", IBCDenom: "ibc/78F7404035221CD1010518C7BC3DD99B90E59C2BA37ABFC3CE56B0CFB7E8901B", Exponent: 8, PriceFeed: true}, // osmosis wBTC
		{Symbol: "EURe", IBCDenom: "ibc/273E4C3B307F3BD56F08652B1DA009A705F078811716DF53F7829A76ED769A8D", Exponent: 6, PriceFeed: false},
		{Symbol: "MARS", IBCDenom: "factory/neutron1ndu2wvkrxtane8se2tr48gv7nsm46y5gcqjhux/MARS", Exponent: 6, PriceFeed: false},
		{Symbol: "ETH", IBCDenom: "ibc/3F1D988D9EEA19EB0F3950B4C19664218031D8BCE68CE7DE30F187D5ACEA0463", Exponent: 8, PriceFeed: true}, // wETH.atom --Eureka
		{Symbol: "TAB", IBCDenom: "factory/neutron1r5qx58l3xx2y8gzjtkqjndjgx69mktmapl45vns0pa73z0zpn7fqgltnll/TAB", Exponent: 6, PriceFeed: false},
		{Symbol: "BTC", IBCDenom: "ibc/0E293A7622DC9A6439DB60E6D234B5AF446962E27CA3AB44D0590603DFF6968E", Exponent: 8, PriceFeed: true}, // wBTC.cosmos
		{Symbol: "MaxBTC", IBCDenom: "factory/neutron17sp75wng9vl2hu3sf4ky86d7smmk3wle9gkts2gmedn9x4ut3xcqa5xp34/maxbtc", Exponent: 8, PriceFeed: false},
	}
)

const (
	SLEEP_TIMEOUT = 30 * time.Second
)

var MinAverageSellPrice = math_utils.MustNewPrecDecFromStr("0.0000000000000000000001")

func SuckUpDust(baseClient *base.BaseClient, grpcConn *grpc.ClientConn) {
	dexClient := dex.CreateClient(grpcConn, baseClient)
	oracleClient := oracle.CreateOracleClient(grpcConn, baseClient)

	for {
		for _, denom := range dustDenoms {

			baseClient.WaitNBlocks(1, time.Second*5)

			var price math_utils.PrecDec
			var err error
			if denom.PriceFeed {
				price, err = oracleClient.GetPrice(denom.Symbol)
				if err != nil {
					log.Errorf("Failed to get price for %v: %v", denom.Symbol, err)
					continue
				}
			} else {

				rawPrice, err := dexClient.GetSpotPrice(denom.IBCDenom, USDC.IBCDenom)
				if err != nil {
					log.Errorf("Failed to get spot price for %v: %v", denom.Symbol, err)
					continue
				}
				exponentDiff := math.Pow(10, float64(denom.Exponent-USDC.Exponent))
				price = rawPrice.Mul(math_utils.MustNewPrecDecFromStr(fmt.Sprintf("%.27f", exponentDiff)))
			}

			exponentDiff := math.Pow(10, float64(USDC.Exponent-denom.Exponent))
			exponentAdjustedSellPrice := price.Mul(math_utils.MustNewPrecDecFromStr(fmt.Sprintf("%.27f", exponentDiff)))

			if rand.Intn(2) == 0 { // Randomly do buy or sell order
				// Buy Denom with USDC
				buyPriceWithSpread := math_utils.OnePrecDec().Quo(exponentAdjustedSellPrice.Mul(math_utils.OnePrecDec().Add(DefaultSpread)))
				AmountIn := math_utils.OnePrecDec().Quo(buyPriceWithSpread).Ceil().TruncateInt().MulRaw(2)

				log.Infof("Buy USDC => %v:  BuyPrice: %v; AmountIn: %v", denom.Symbol, buyPriceWithSpread, AmountIn)
				resp, err := dexClient.PlaceLimitOrder(USDC.IBCDenom, denom.IBCDenom, AmountIn, buyPriceWithSpread, dextypes.LimitOrderType_IMMEDIATE_OR_CANCEL, MinAverageSellPrice)
				if err != nil {
					log.Errorf("Failed to place limit order: %v", err)

				} else {
					log.Infof("USDC => %v success: %v", denom.Symbol, resp.TxResponse.TxHash)
				}
			} else {
				// Sell Denom for USDC
				sellPriceWithSpread := exponentAdjustedSellPrice.Mul(math_utils.OnePrecDec().Sub(DefaultSpread))
				AmountIn := math_utils.OnePrecDec().Quo(sellPriceWithSpread).Ceil().TruncateInt().MulRaw(2)
				log.Infof("Sell %v => USDC:  SellPrice: %v; AmountIn: %v", denom.Symbol, sellPriceWithSpread, AmountIn)
				resp, err := dexClient.PlaceLimitOrder(denom.IBCDenom, USDC.IBCDenom, AmountIn, sellPriceWithSpread, dextypes.LimitOrderType_IMMEDIATE_OR_CANCEL, MinAverageSellPrice)
				if err != nil {
					log.Errorf("Failed to place limit order: %v", err)
				} else {
					log.Infof("%v => USDC success: %v", denom.Symbol, resp.TxResponse.TxHash)
				}
			}

		}

		time.Sleep(SLEEP_TIMEOUT)
	}

}
