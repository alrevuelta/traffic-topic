package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/p2p/enode"
	logging "github.com/ipfs/go-log/v2"
	"github.com/jackc/pgx/v4"
	logrus "github.com/sirupsen/logrus"
	"github.com/waku-org/go-waku/waku/v2/dnsdisc"
	"github.com/waku-org/go-waku/waku/v2/node"
	"github.com/waku-org/go-waku/waku/v2/utils"
	"go.uber.org/zap"
)

// var TopicToBytes map[string]uint64 //use a thread safe one
var TopicToBytes sync.Map
var TopicToCount sync.Map

var log = utils.Logger().Named("basic2")

var Db *pgx.Conn

var Table = `
CREATE TABLE IF NOT EXISTS t_ctopic_traffic (
	 f_timestamp TIMESTAMPTZ,
	 f_content_topic TEXT,
	 f_bytes_sent NUMERIC,
	 PRIMARY KEY (f_timestamp, f_content_topic)
);
`

var Insert = `
INSERT INTO t_ctopic_traffic(
	f_timestamp,
	f_content_topic,
	f_bytes_sent)
VALUES ($1, $2, $3)
`

func main() {
	customFormatter := new(logrus.TextFormatter)
	customFormatter.TimestampFormat = "2006-01-02 15:04:05"
	customFormatter.FullTimestamp = true
	logrus.SetFormatter(customFormatter)

	cfg, err := NewCliConfig()
	if err != nil {
		log.Fatal("error parsing config", zap.Error(err))
	}

	//postgresEndpoint := "postgres://xxx:yyy@localhost:5432"
	if !cfg.DryRun {
		Db, err = pgx.Connect(context.Background(), cfg.PostgresEndpoint)

		if err != nil {
			log.Fatal("Could not connect to postgres", zap.Error(err))
		}
		if cfg.ResetTable {
			logrus.Info("Reseting table t_ctopic_traffic")
			_, err = Db.Exec(context.Background(), "drop table if exists t_ctopic_traffic")
			if err != nil {
				log.Fatal("error cleaning table t_oracle_validator_balances at startup: ", zap.Error(err))
			}
		}
		// Create table if it doesnt exist
		if _, err := Db.Exec(
			context.Background(),
			Table); err != nil {
			log.Fatal("error creating table t_oracle_validator_balances: ", zap.Error(err))
		}
	}

	lvl, err := logging.LevelFromString("warn")
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

	discoveryURL := "enrtree://AOGECG2SPND25EEFMAJ5WF3KSGJNSGV356DSTL2YVLLZWIV6SAYBM@prod.nodes.status.im"
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
		node.WithDiscoverParams(30),
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
	go heartBeat(cfg.TickSeconds, cfg.DryRun)

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

func heartBeat(tickSeconds int, dryRun bool) {

	for range time.Tick(time.Second * time.Duration(tickSeconds)) {

		// Save traffic to db
		nowTimeStamp := time.Now()

		bytesSent := uint64(0)
		TopicToBytes.Range(func(k, v interface{}) bool {
			if !dryRun {
				logrus.Info("Saving content to db: ", nowTimeStamp)
				_, err := Db.Exec(
					context.Background(),
					Insert,
					nowTimeStamp,
					k.(string),
					v.(uint64))
				if err != nil {
					log.Error("error inserting into table t_ctopic_traffic: ", zap.Error(err))
				}
			}
			bytesSent += v.(uint64)
			//fmt.Println("range (): ", k, " ", v)
			return true
		})
		//printSummary(TopicToBytes, TopicToCount)

		logrus.Info("Total bytes sent: ", bytesSent, " in ", nowTimeStamp, " during ", tickSeconds, " seconds")

		// Clean map for next iteration.
		TopicToBytes.Range(func(key interface{}, value interface{}) bool {
			TopicToBytes.Delete(key)
			return true
		})
	}
}

func readLoop(ctx context.Context, wakuNode *node.WakuNode) {

	sub, err := wakuNode.Relay().SubscribeToTopic(ctx, "/waku/2/default-waku/proto")
	if err != nil {
		log.Error("Could not subscribe", zap.Error(err))
		return
	}

	for value := range sub.C {
		msg := value.Message()
		//logrus.Info("Received msg", msg.ContentTopic, " ", msg.Ephemeral, " ", len(msg.Payload))
		payloadSizeInBytes := len(msg.Payload)
		if payloadSizeInBytes > 10000 {
			logrus.Info("Big Payload", msg.ContentTopic, " ephemeral=", msg.Ephemeral, " ", len(msg.Payload), " bytes")
		}

		//TopicToBytes[msg.ContentTopic] += len(msg.Payload)

		// If topic already exists increase, otherwise set.
		value, ok := TopicToBytes.Load(msg.ContentTopic)
		if ok {
			TopicToBytes.Store(msg.ContentTopic, value.(uint64)+uint64(len(msg.Payload)))
		} else {
			TopicToBytes.Store(msg.ContentTopic, uint64(len(msg.Payload)))
		}

		topicCount, ok := TopicToCount.Load(msg.ContentTopic)
		if ok {
			TopicToCount.Store(msg.ContentTopic, topicCount.(uint64)+uint64(1))
		} else {
			TopicToCount.Store(msg.ContentTopic, uint64(1))
		}

		/*
			payload, err := payload.DecodePayload(value.Message(), &payload.KeyInfo{Kind: payload.None})
			if err != nil {
				logrus.Info("Could not decode", " ", len(value.Message().Payload), " , ", value.Message().ContentTopic)
				continue
			}

		*/

	}
}

func printSummary(mapTopicToBytes sync.Map, mapTopicToCount sync.Map) {
	mapTopicToBytes.Range(func(k, v interface{}) bool {
		logrus.Info("1range (): ", k, " ", v)
		return true
	})
	mapTopicToCount.Range(func(k, v interface{}) bool {
		logrus.Info("2range (): ", k, " ", v)
		return true
	})

	//logrus.Info("Total bytes sent: ", bytesSent, " in ", nowTimeStamp, " during ", tickSeconds, " seconds")

}
