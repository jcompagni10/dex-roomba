package main

import (
	"context"

	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/jcompagni10/dex-roomba/x/dex"
	"google.golang.org/grpc"
)

func main() {

	grpcConn, err := grpc.NewClient(
		"grpc-falcron.pion-1.ntrn.tech:80",
		grpc.WithInsecure(),
		grpc.WithDefaultCallOptions(grpc.ForceCodec(codec.NewProtoCodec(nil).GRPCCodec())),
	)
	if err != nil {
		panic(err)
	}
	defer grpcConn.Close()

	dex.PlaceLimitOrder(context.Background(), grpcConn)

}
