package roomba

import (
	"fmt"
	"math"
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
	Symbol   string
	IBCDenom string
	Exponent int64
}

var (
	DefaultSpread = math_utils.MustNewPrecDecFromStr("0.01")
	USDC          = Denom{Symbol: "USDC", IBCDenom: "ibc/B559A80D62249C8AA07A380E2A2BEA6E5CA9A6F079C912C3A9E9B494105E4F81", Exponent: 6}
	dustDenoms    = []Denom{
		{Symbol: "NTRN", IBCDenom: "untrn", Exponent: 6},
		{Symbol: "ATOM", IBCDenom: "ibc/C4CFF46FD6DE35CA4CF4CE031E643C8FDC9BA4B99AE598E9B0ED98FE3A2319F9", Exponent: 6},
		{Symbol: "TIA", IBCDenom: "ibc/773B4D0A3CD667B2275D5A4A7A2F0909C0BA0F4059C0B9181E680DDF4965DCC7", Exponent: 6},
		{Symbol: "BTC", IBCDenom: "ibc/DF8722298D192AAB85D86D0462E8166234A6A9A572DD4A2EA7996029DF4DB363", Exponent: 8},
		{Symbol: "DYDX", IBCDenom: "ibc/2CB87BCE0937B1D1DFCEE79BE4501AAF3C265E923509AEAC410AD85D27F35130", Exponent: 18},
		{Symbol: "OSMO", IBCDenom: "ibc/376222D6D9DAE23092E29740E56B758580935A6D77C24C2ABD57A6A78A1F3955", Exponent: 6},
		{Symbol: "ETH", IBCDenom: "ibc/A585C2D15DCD3B010849B453A2CFCB5E213208A5AB665691792684C26274304D", Exponent: 18},
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

			price, err := oracleClient.GetPrice(denom.Symbol)
			if err != nil {
				log.Errorf("Failed to get price: %v", err)
				continue
			}
			exponentDiff := math.Pow(10, float64(USDC.Exponent-denom.Exponent))
			exponentAdjustedPrice := price.Mul(math_utils.MustNewPrecDecFromStr(fmt.Sprintf("%.27f", exponentDiff)))

			// Buy Denom with USDC
			priceWithSpread := math_utils.OnePrecDec().Quo(exponentAdjustedPrice.Mul(math_utils.OnePrecDec().Add(DefaultSpread)))
			AmountIn := math_utils.OnePrecDec().Quo(priceWithSpread).Ceil().TruncateInt().MulRaw(2)

			log.Infof("Sell USDC => %v:  SellPrice: %v; AmountIn: %v", denom.Symbol, priceWithSpread, AmountIn)
			resp, err := dexClient.PlaceLimitOrder(USDC.IBCDenom, denom.IBCDenom, AmountIn, priceWithSpread, dextypes.LimitOrderType_IMMEDIATE_OR_CANCEL, MinAverageSellPrice)
			if err != nil {
				log.Errorf("Failed to place limit order: %v", err)

			} else {
				log.Infof("USDC => %v success: %v", denom.Symbol, resp.TxResponse.TxHash)
			}

			baseClient.WaitNBlocks(1, time.Second*5)

			// Sell Denom for USDC
			priceWithSpread = exponentAdjustedPrice.Mul(math_utils.OnePrecDec().Sub(DefaultSpread))
			AmountIn = math_utils.OnePrecDec().Quo(priceWithSpread).Ceil().TruncateInt().MulRaw(2)
			log.Infof("Sell %v => USDC:  SellPrice: %v; AmountIn: %v", denom.Symbol, priceWithSpread, AmountIn)
			resp, err = dexClient.PlaceLimitOrder(denom.IBCDenom, USDC.IBCDenom, AmountIn, priceWithSpread, dextypes.LimitOrderType_IMMEDIATE_OR_CANCEL, MinAverageSellPrice)
			if err != nil {
				log.Errorf("Failed to place limit order: %v", err)
			} else {
				log.Infof("%v => USDC success: %v", denom.Symbol, resp.TxResponse.TxHash)
			}
		}
		time.Sleep(SLEEP_TIMEOUT)
	}

}
