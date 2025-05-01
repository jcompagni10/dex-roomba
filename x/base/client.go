package base

import (
	"context"
	"fmt"
	"math"
	"os"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/cosmos/cosmos-sdk/client"
	txclient "github.com/cosmos/cosmos-sdk/client/tx"
	"github.com/cosmos/cosmos-sdk/crypto/hd"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/tx"
	txsigning "github.com/cosmos/cosmos-sdk/types/tx/signing"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/neutron-org/neutron/v6/app"
	"github.com/neutron-org/neutron/v6/app/config"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	APP_KEY_NAME           = "app-key"
	UNTRN_GAS_PRICE        = 0.0053
	DEFAULT_GAS_ADJUSTMENT = 1.5
)

var (
	CHAIN_ID     = os.Getenv("CHAIN_ID")
	RPC_ENDPOINT = os.Getenv("RPC_ENDPOINT")
)

type BaseClient struct {
	txClient   tx.ServiceClient
	Address    string
	ClientCtx  client.Context
	authClient authtypes.QueryClient
}

func CreateGRPCConn(grpcEndpoint string) *grpc.ClientConn {
	log.Printf("Connecting to gRPC endpoint: %s", grpcEndpoint)

	grpcConn, err := grpc.Dial(
		grpcEndpoint,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.Fatalf("Failed to connect to gRPC server: %v", err)
	}

	return grpcConn
}

func CreateClient(conn *grpc.ClientConn) *BaseClient {

	config.GetDefaultConfig()

	encodingConfig := app.MakeEncodingConfig()

	txClient := tx.NewServiceClient(conn)
	authClient := authtypes.NewQueryClient(conn)

	kb := keyring.NewInMemory(encodingConfig.Marshaler)

	mnemonic := os.Getenv("ACCOUNT_MNEMONIC")
	key, err := kb.NewAccount(
		APP_KEY_NAME,
		mnemonic,
		"",
		hd.CreateHDPath(118, 0, 0).String(),
		hd.Secp256k1,
	)
	if err != nil {
		panic(err)
	}

	address, err := key.GetAddress()
	if err != nil {
		panic(err)
	}

	rpcClient, err := client.NewClientFromNode(RPC_ENDPOINT)

	clientCtx := client.Context{}.
		WithChainID(CHAIN_ID).
		WithKeyring(kb).
		WithInterfaceRegistry(encodingConfig.InterfaceRegistry).
		WithBroadcastMode("sync").
		WithGRPCClient(conn).
		WithTxConfig(encodingConfig.TxConfig).
		WithClient(rpcClient)

	return &BaseClient{
		txClient:   txClient,
		Address:    address.String(),
		ClientCtx:  clientCtx,
		authClient: authClient,
	}
}

func (c *BaseClient) GetAccount(address string) (*authtypes.BaseAccount, error) {
	accountResp, err := c.authClient.Account(
		context.Background(),
		&authtypes.QueryAccountRequest{Address: c.Address})
	if err != nil {
		return nil, err
	}

	var acc sdk.AccountI
	if err := c.ClientCtx.InterfaceRegistry.UnpackAny(accountResp.Account, &acc); err != nil {
		return nil, err
	}

	return acc.(*authtypes.BaseAccount), nil
}

func (c *BaseClient) SendTx(msg sdk.Msg, getResponse bool) (*tx.GetTxResponse, error) {

	baseAcc, err := c.GetAccount(c.Address)
	if err != nil {
		return nil, err
	}

	gasUsed, err := c.SimulateTx(baseAcc, msg)
	if err != nil {
		return nil, err
	}

	gasToUse := math.Ceil(float64(gasUsed) * DEFAULT_GAS_ADJUSTMENT)
	fees := fmt.Sprintf("%funtrn", math.Ceil(UNTRN_GAS_PRICE*float64(gasToUse)))
	txBytes, err := c.BuildAndEncodeTx(baseAcc, msg, fees, uint64(gasToUse))
	if err != nil {
		return nil, err
	}

	res, err := c.txClient.BroadcastTx(context.Background(), &tx.BroadcastTxRequest{
		Mode:    tx.BroadcastMode_BROADCAST_MODE_SYNC,
		TxBytes: txBytes,
	})
	if err != nil {
		return nil, err
	}
	if res.TxResponse.Code != 0 {
		return nil, fmt.Errorf("tx failed with code %d: %v", res.TxResponse.Code, res.TxResponse.RawLog)
	}

	if getResponse {
		err := c.WaitNBlocks(2, 8*time.Second)
		if err != nil {
			return nil, err
		}
		txResp, err := c.QueryTx(res.TxResponse.TxHash)

		if err != nil {
			return nil, err
		}
		if txResp.TxResponse.Code != 0 {
			return nil, fmt.Errorf("tx failed with code %d: %v", txResp.TxResponse.Code, txResp.TxResponse.RawLog)
		}
		if txResp.TxResponse.RawLog != "" {
			return nil, fmt.Errorf("tx failed: %v", txResp.TxResponse.RawLog)
		}
		return txResp, nil
	} else {
		return nil, nil
	}
}

func (c *BaseClient) SimulateTx(baseAcc *authtypes.BaseAccount, msg sdk.Msg) (uint64, error) {
	txBytes, err := c.BuildAndEncodeTx(baseAcc, msg, "", 0)
	if err != nil {
		return 0, err
	}

	res, err := c.txClient.Simulate(context.Background(), &tx.SimulateRequest{
		TxBytes: txBytes,
	})
	if err != nil {
		return 0, err
	}

	// TODO: check if tx succeeds
	return res.GasInfo.GasUsed, nil
}

func (c *BaseClient) BuildAndEncodeTx(baseAcc *authtypes.BaseAccount, msg sdk.Msg, fees string, gas uint64) ([]byte, error) {
	txf := txclient.Factory{}.
		WithChainID(CHAIN_ID).
		WithKeybase(c.ClientCtx.Keyring).
		WithFees(fees).
		WithGas(gas).
		WithMemo("DexRoomba").
		WithAccountNumber(baseAcc.AccountNumber).
		WithSequence(baseAcc.Sequence).
		WithSignMode(txsigning.SignMode_SIGN_MODE_DIRECT).
		WithTxConfig(c.ClientCtx.TxConfig)

	txBuilder, err := txf.BuildUnsignedTx(msg)
	if err != nil {
		return nil, err
	}

	err = txclient.Sign(c.ClientCtx.CmdContext, txf, APP_KEY_NAME, txBuilder, true)
	if err != nil {
		return nil, err
	}

	txBytes, err := c.ClientCtx.TxConfig.TxEncoder()(txBuilder.GetTx())
	if err != nil {
		return nil, err
	}

	return txBytes, nil
}

func (c *BaseClient) QueryTx(txHash string) (*tx.GetTxResponse, error) {
	txResp, err := c.txClient.GetTx(context.Background(), &tx.GetTxRequest{
		Hash: txHash,
	})
	if err != nil {
		return nil, err
	}

	return txResp, nil
}

func (c *BaseClient) WaitNBlocks(n int64, timeout time.Duration) error {

	startHeight, err := c.GetLatestBlockHeight()
	if err != nil {
		return fmt.Errorf("failed to get latest block height: %w", err)
	}

	startTime := time.Now()
	for {
		if time.Since(startTime) > timeout {
			return fmt.Errorf("timed out waiting for %d blocks", n)
		}

		newHeight, err := c.GetLatestBlockHeight()
		if err != nil {
			return fmt.Errorf("failed to get latest block height: %w", err)
		}

		if newHeight-startHeight >= n {
			return nil
		}

		time.Sleep(1 * time.Second)
	}
}

func (c *BaseClient) GetLatestBlockHeight() (int64, error) {
	res, err := c.ClientCtx.Client.ABCIInfo(context.Background())
	if err != nil {
		return 0, err
	}

	return res.Response.LastBlockHeight, nil
}
