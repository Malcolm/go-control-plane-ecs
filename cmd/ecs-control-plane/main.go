package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"time"

	"google.golang.org/grpc"

	"github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	"github.com/envoyproxy/go-control-plane/pkg/server/v3"
	"github.com/envoyproxy/go-control-plane/pkg/test/v3"
	discoverygrpc "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
)

// Logger is a logger that implements the pkg/log/Logger interface.
type Logger struct {
	Debug bool
}

// Debugf logs a formatted debug message.
func (l Logger) Debugf(format string, args ...interface{}) {
	if l.Debug {
		log.Printf(format, args...)
	}
}

// Infof logs a formatted info message.
func (l Logger) Infof(format string, args ...interface{}) {
	log.Printf(format, args...)
}

// Warnf logs a formatted warning message.
func (l Logger) Warnf(format string, args ...interface{}) {
	log.Printf(format, args...)
}

// Errorf logs a formatted error message.
func (l Logger) Errorf(format string, args ...interface{}) {
	log.Printf(format, args...)
}

var (
	l               Logger
	port            uint
	nodeID          string
	awsRegion       string
	ecsClusterName  string
	ecsServiceName  string
	pollingInterval time.Duration
)

func init() {
	flag.BoolVar(&l.Debug, "debug", false, "Enable xDS server debug logging")
	flag.UintVar(&port, "port", 18000, "xDS management server port")
	flag.StringVar(&nodeID, "nodeID", "test-id", "Node ID")
	flag.StringVar(&awsRegion, "aws-region", "us-west-2", "AWS region")
	flag.StringVar(&ecsClusterName, "ecs-cluster", "", "ECS cluster name")
	flag.StringVar(&ecsServiceName, "ecs-service", "", "ECS service name")
	flag.DurationVar(&pollingInterval, "polling-interval", 10*time.Second, "Polling interval for ECS metadata")
}

// RunServer starts an xDS server at the given port.
func RunServer(srv server.Server, port uint) {
	grpcServer := grpc.NewServer()
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		log.Fatal(err)
	}

	discoverygrpc.RegisterAggregatedDiscoveryServiceServer(grpcServer, srv)

	log.Printf("management server listening on %d\n", port)
	if err := grpcServer.Serve(lis); err != nil {
		log.Println(err)
	}
}

func main() {
	flag.Parse()

	if ecsClusterName == "" {
		log.Fatal("ECS cluster name is required")
	}
	if ecsServiceName == "" {
		log.Fatal("ECS service name is required")
	}

	// Create a cache
	cache := cache.NewSnapshotCache(false, cache.IDHash{}, l)

	// Start the ECS poller goroutine
	go runEcsPoller(context.Background(), cache, awsRegion, ecsClusterName, ecsServiceName, nodeID, pollingInterval, l)

	// Run the xDS server
	ctx := context.Background()
	cb := &test.Callbacks{Debug: l.Debug}
	srv := server.NewServer(ctx, cache, cb)
	RunServer(srv, port)
}
