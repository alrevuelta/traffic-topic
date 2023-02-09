package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/p2p/enode"
	logging "github.com/ipfs/go-log/v2"
	"github.com/waku-org/go-waku/waku/v2/dnsdisc"
	"github.com/waku-org/go-waku/waku/v2/node"
	"github.com/waku-org/go-waku/waku/v2/payload"
	"github.com/waku-org/go-waku/waku/v2/utils"
	"go.uber.org/zap"
)

var log = utils.Logger().Named("basic2")

func main() {
	lvl, err := logging.LevelFromString("info")
	if err != nil {
		panic(err)
	}
	logging.SetAllLoggers(lvl)

	hostAddr, _ := net.ResolveTCPAddr("tcp", "0.0.0.0:0")
	key, err := randomHex(32)
	if err != nil {
		log.Error("Could not generate random key", zap.Error(err))
		return
	}
	prvKey, err := crypto.HexToECDSA(key)
	if err != nil {
		log.Error("Could not convert hex into ecdsa key", zap.Error(err))
		return
	}

	ctx := context.Background()

	discoveryURL := "enrtree://AOGECG2SPND25EEFMAJ5WF3KSGJNSGV356DSTL2YVLLZWIV6SAYBM@test.waku.nodes.status.im"
	nodes, err := dnsdisc.RetrieveNodes(context.Background(), discoveryURL)
	if err != nil {
		panic(err)
	}

	fmt.Println(nodes)

	wakuNode, err := node.New(
		node.WithPrivateKey(prvKey),
		node.WithHostAddress(hostAddr),
		node.WithNTP(),
		node.WithWakuRelay(),
		// TODO: add all
		node.WithDiscoveryV5(8000, []*enode.Node{nodes[0].ENR}, true),
		node.WithDiscoverParams(10),
	)
	if err != nil {
		log.Error("Error creating wakunode", zap.Error(err))
		return
	}

	if err := wakuNode.Start(ctx); err != nil {
		log.Error("Error starting wakunode", zap.Error(err))
		return
	}

	err = wakuNode.DiscV5().Start(ctx)
	if err != nil {
		log.Fatal("Error starting discovery", zap.Error(err))
	}

	go readLoop(ctx, wakuNode)

	// Wait for a SIGINT or SIGTERM signal
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch
	fmt.Println("\n\n\nReceived signal, shutting down...")

	// shut the node down
	wakuNode.Stop()

}

func randomHex(n int) (string, error) {
	bytes := make([]byte, n)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func readLoop(ctx context.Context, wakuNode *node.WakuNode) {

	sub, err := wakuNode.Relay().SubscribeToTopic(ctx, "/waku/2/default-waku/proto")
	if err != nil {
		log.Error("Could not subscribe", zap.Error(err))
		return
	}

	for value := range sub.C {
		log.Info("rx ", zap.String("ContentTopic", value.Message().ContentTopic))
		payload, err := payload.DecodePayload(value.Message(), &payload.KeyInfo{Kind: payload.None})
		if err != nil {
			fmt.Println(err)
			continue
		}
		log.Info("Received msg, ", zap.String("data", string(payload.Data)))
	}
}
