package dex

import (
	"context"
	"fmt"

	sdkmath "cosmossdk.io/math"

	sdkquery "github.com/cosmos/cosmos-sdk/types/query"
	"github.com/cosmos/cosmos-sdk/types/tx"
	"github.com/jcompagni10/dex-roomba/x/base"
	math_utils "github.com/neutron-org/neutron/v6/utils/math"
	dextypes "github.com/neutron-org/neutron/v6/x/dex/types"

	"google.golang.org/grpc"
)

type DexClient struct {
	baseClient  *base.BaseClient
	client      dextypes.MsgClient
	QueryClient dextypes.QueryClient
}

func CreateClient(conn *grpc.ClientConn, baseClient *base.BaseClient) *DexClient {

	msgClient := dextypes.NewMsgClient(conn)

	queryClient := dextypes.NewQueryClient(conn)

	return &DexClient{
		client:      msgClient,
		QueryClient: queryClient,
		baseClient:  baseClient,
	}
}

func (c *DexClient) PlaceLimitOrder(
	tokenIn string,
	tokenOut string,
	amountIn sdkmath.Int,
	limitSellPrice math_utils.PrecDec,
	orderType dextypes.LimitOrderType,
	minAverageSellPrice math_utils.PrecDec,
) (*tx.GetTxResponse, error) {
	msg := &dextypes.MsgPlaceLimitOrder{
		Creator:             c.baseClient.Address,
		Receiver:            c.baseClient.Address,
		TokenIn:             tokenIn,
		TokenOut:            tokenOut,
		OrderType:           orderType,
		AmountIn:            amountIn,
		LimitSellPrice:      &limitSellPrice,
		MinAverageSellPrice: &minAverageSellPrice,
	}

	resp, err := c.baseClient.SendTx(msg, true)
	if err != nil {
		return nil, err
	}
	return resp, nil

}

func (c *DexClient) GetSpotPrice(
	tokenIn string,
	tokenOut string,
) (math_utils.PrecDec, error) {

	resp, err := c.QueryClient.TickLiquidityAll(context.Background(), &dextypes.QueryAllTickLiquidityRequest{
		PairId:  dextypes.MustNewPairID(tokenIn, tokenOut).CanonicalString(),
		TokenIn: tokenIn,
		Pagination: &sdkquery.PageRequest{
			Limit: 1,
		},
	})
	if err != nil {
		fmt.Printf("res: %v, req: %v", resp, &dextypes.QueryAllTickLiquidityRequest{
			PairId:  dextypes.MustNewPairID(tokenIn, tokenOut).CanonicalString(),
			TokenIn: tokenIn,
			Pagination: &sdkquery.PageRequest{
				Limit: 1,
			},
		})
		return math_utils.ZeroPrecDec(), err
	}

	if len(resp.TickLiquidity) == 0 {
		return math_utils.ZeroPrecDec(), fmt.Errorf("no tick liquidity found")
	}

	return resp.TickLiquidity[0].Price(), nil
}
