package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"strings"
	"time"

	"github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	"github.com/envoyproxy/go-control-plane/pkg/server/v3"
	"github.com/envoyproxy/go-control-plane/pkg/test/v3"
	discoverygrpc "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	pflag "github.com/spf13/pflag"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
)

// Config holds the application's configuration.
type Config struct {
	Port            uint          `mapstructure:"port"`
	NodeID          string        `mapstructure:"node-id"`
	AWSRegion       string        `mapstructure:"aws-region"`
	ECSCluster      string        `mapstructure:"ecs-cluster"`
	ECSService      string        `mapstructure:"ecs-service"`
	PollingInterval time.Duration `mapstructure:"polling-interval"`
}

type simpleLogger struct{}

func (l simpleLogger) Debugf(format string, args ...interface{}) { log.Printf(format, args...) }
func (l simpleLogger) Infof(format string, args ...interface{})  { log.Printf(format, args...) }
func (l simpleLogger) Warnf(format string, args ...interface{})   { log.Printf(format, args...) }
func (l simpleLogger) Errorf(format string, args ...interface{}) { log.Printf(format, args...) }

// loadConfig loads configuration from file, environment variables, and flags.
func loadConfig(logger simpleLogger) (*Config, error) {
	v := viper.New()

	v.SetDefault("port", 18000)
	v.SetDefault("node-id", "test-id")
	v.SetDefault("polling-interval", 10*time.Second)

	v.BindPFlags(pflag.CommandLine)
	v.SetEnvPrefix("XDS")
	v.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	v.AutomaticEnv()

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func main() {
	pflag.Uint("port", 18000, "xDS management server port")
	pflag.String("node-id", "test-id", "Node ID")
	pflag.String("aws-region", "us-west-2", "AWS region")
	pflag.String("ecs-cluster", "", "ECS cluster name")
	pflag.String("ecs-service", "", "ECS service name")
	pflag.Duration("polling-interval", 10*time.Second, "Polling interval for ECS metadata")
	pflag.Parse()

	logger := simpleLogger{}
	cfg, err := loadConfig(logger)
	if err != nil {
		log.Fatalf("failed to load configuration: %v", err)
	}

	if cfg.ECSCluster == "" || cfg.ECSService == "" {
		log.Fatal("'-ecs-cluster' and '-ecs-service' are required")
	}

	snapshotCache := cache.NewSnapshotCache(false, cache.IDHash{}, logger)

	go runEcsPoller(context.Background(), snapshotCache, cfg.AWSRegion, cfg.ECSCluster, cfg.ECSService, cfg.NodeID, cfg.PollingInterval, logger)

	ctx := context.Background()
	cb := &test.Callbacks{}
	srv := server.NewServer(ctx, snapshotCache, cb)

	grpcServer := grpc.NewServer()
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.Port))
	if err != nil {
		log.Fatal(err)
	}

	discoverygrpc.RegisterAggregatedDiscoveryServiceServer(grpcServer, srv)
	logger.Infof("management server listening on %d", cfg.Port)
	if err := grpcServer.Serve(lis); err != nil {
		log.Println(err)
	}
}
