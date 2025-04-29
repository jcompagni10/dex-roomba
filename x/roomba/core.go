package roomba

import (
	"math"
	"time"

	log "github.com/sirupsen/logrus"

	sdkmath "cosmossdk.io/math"
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
	DefaultSpread = math_utils.MustNewPrecDecFromStr("0.005")
	USDC          = Denom{Symbol: "USDC", IBCDenom: "ibc/B559A80D62249C8AA07A380E2A2BEA6E5CA9A6F079C912C3A9E9B494105E4F81", Exponent: 6}
	dustDenoms    = []Denom{
		{Symbol: "NTRN", IBCDenom: "untrn", Exponent: 6},
		{Symbol: "ATOM", IBCDenom: "ibc/C4CFF46FD6DE35CA4CF4CE031E643C8FDC9BA4B99AE598E9B0ED98FE3A2319F9", Exponent: 6},
		{Symbol: "TIA", IBCDenom: "ibc/773B4D0A3CD667B2275D5A4A7A2F0909C0BA0F4059C0B9181E680DDF4965DCC7", Exponent: 6},
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
			exponentDiff := USDC.Exponent - denom.Exponent
			exponentAdjustedPrice := price.MulInt64(int64(math.Pow(float64(10), float64(exponentDiff))))

			// Buy Denom with USDC
			priceWithSpread := math_utils.OnePrecDec().Quo(exponentAdjustedPrice.Mul(math_utils.OnePrecDec().Add(DefaultSpread)))
			resp, err := dexClient.PlaceLimitOrder(USDC.IBCDenom, denom.IBCDenom, sdkmath.NewInt(10), priceWithSpread, dextypes.LimitOrderType_IMMEDIATE_OR_CANCEL, MinAverageSellPrice)
			if err != nil {
				log.Errorf("Failed to place limit order: %v", err)

			} else {
				log.Infof("Buy Denom %v with USDC: %v", denom.Symbol, resp.TxResponse.TxHash)
			}

			baseClient.WaitNBlocks(1, time.Second*5)

			// Sell Denom for USDC
			priceWithSpread = exponentAdjustedPrice.Mul(math_utils.OnePrecDec().Sub(DefaultSpread))
			resp, err = dexClient.PlaceLimitOrder(denom.IBCDenom, USDC.IBCDenom, sdkmath.NewInt(10), priceWithSpread, dextypes.LimitOrderType_IMMEDIATE_OR_CANCEL, MinAverageSellPrice)
			if err != nil {
				log.Errorf("Failed to place limit order: %v", err)
			} else {
				log.Infof("Sell %v for USDC: %v", denom.Symbol, resp.TxResponse.TxHash)
			}
		}
		time.Sleep(SLEEP_TIMEOUT)
	}

}
