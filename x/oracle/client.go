package oracle

import (
	"context"

	"github.com/jcompagni10/dex-roomba/x/base"
	math_utils "github.com/neutron-org/neutron/v6/utils/math"
	slinkytypes "github.com/skip-mev/slinky/pkg/types"
	"github.com/skip-mev/slinky/x/oracle/types"
	"google.golang.org/grpc"
)

type OracleClient struct {
	baseClient   *base.BaseClient
	oracleClient types.QueryClient
}

func CreateOracleClient(grpcConn *grpc.ClientConn, baseClient *base.BaseClient) *OracleClient {
	return &OracleClient{
		baseClient:   baseClient,
		oracleClient: types.NewQueryClient(grpcConn),
	}
}

func (c *OracleClient) GetPrice(symbol string) (math_utils.PrecDec, error) {
	resp, err := c.oracleClient.GetPrice(context.Background(), &types.GetPriceRequest{
		CurrencyPair: slinkytypes.CurrencyPair{
			Base:  symbol,
			Quote: "USD",
		},
	})
	if err != nil {
		return math_utils.ZeroPrecDec(), err
	}

	price := math_utils.NewPrecDecFromIntWithPrec(resp.Price.Price, int64(resp.Decimals))
	return price, nil
}
