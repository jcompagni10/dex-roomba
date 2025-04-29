package main

import (
	"os"

	"github.com/jcompagni10/dex-roomba/x/base"
	"github.com/jcompagni10/dex-roomba/x/roomba"
)

var (
	GRPC_ENDPOINT = os.Getenv("GRPC_ENDPOINT")
	CHAIN_ID      = os.Getenv("CHAIN_ID")
)

func main() {
	// Create gRPC connection
	grpcConn := base.CreateGRPCConn(GRPC_ENDPOINT)
	defer grpcConn.Close()

	baseClient := base.CreateClient(grpcConn)

	roomba.SuckUpDust(baseClient, grpcConn)
}
