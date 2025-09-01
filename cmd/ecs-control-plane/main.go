package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"time"

	"github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	"github.com/envoyproxy/go-control-plane/pkg/server/v3"
	"github.com/envoyproxy/go-control-plane/pkg/test/v3"
	discoverygrpc "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	"google.golang.org/grpc"
)

type simpleLogger struct{}

func (l simpleLogger) Debugf(format string, args ...interface{}) { log.Printf(format, args...) }
func (l simpleLogger) Infof(format string, args ...interface{})  { log.Printf(format, args...) }
func (l simpleLogger) Warnf(format string, args ...interface{})   { log.Printf(format, args...) }
func (l simpleLogger) Errorf(format string, args ...interface{}) { log.Printf(format, args...) }

func main() {
	var port uint
	var nodeID, awsRegion, ecsCluster, ecsService string
	var pollingInterval time.Duration

	flag.UintVar(&port, "port", 18000, "xDS management server port")
	flag.StringVar(&nodeID, "node-id", "test-id", "Node ID")
	flag.StringVar(&awsRegion, "aws-region", "us-west-2", "AWS region")
	flag.StringVar(&ecsCluster, "ecs-cluster", "", "ECS cluster name")
	flag.StringVar(&ecsService, "ecs-service", "", "ECS service name")
	flag.DurationVar(&pollingInterval, "polling-interval", 10*time.Second, "Polling interval for ECS metadata")
	flag.Parse()

	if ecsCluster == "" || ecsService == "" {
		log.Fatal("'-ecs-cluster' and '-ecs-service' are required")
	}

	logger := simpleLogger{}
	snapshotCache := cache.NewSnapshotCache(false, cache.IDHash{}, logger)

	go runEcsPoller(context.Background(), snapshotCache, awsRegion, ecsCluster, ecsService, nodeID, pollingInterval, logger)

	ctx := context.Background()
	cb := &test.Callbacks{}
	srv := server.NewServer(ctx, snapshotCache, cb)

	grpcServer := grpc.NewServer()
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		log.Fatal(err)
	}

	discoverygrpc.RegisterAggregatedDiscoveryServiceServer(grpcServer, srv)
	logger.Infof("management server listening on %d", port)
	if err := grpcServer.Serve(lis); err != nil {
		log.Println(err)
	}
}
