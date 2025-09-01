package main

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	discoverygrpc "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	"github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	"github.com/envoyproxy/go-control-plane/pkg/server/v3"
	"github.com/envoyproxy/go-control-plane/pkg/test/v3"
	pflag "github.com/spf13/pflag"
	"github.com/spf13/viper"
	"go.uber.org/zap"
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
	LogLevel        string        `mapstructure:"log-level"`
}

// loadConfig loads configuration from file, environment variables, and flags.
func loadConfig() (*Config, error) {
	v := viper.New()

	v.SetDefault("port", 18000)
	v.SetDefault("node-id", "test-id")
	v.SetDefault("polling-interval", 10*time.Second)
	v.SetDefault("log-level", "info")

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
	pflag.String("log-level", "info", "Log level (debug, info, warn, error)")
	pflag.Parse()

	// A temporary logger for startup.
	tempLogger, _ := zap.NewProduction()
	sugaredTempLogger := tempLogger.Sugar()

	cfg, err := loadConfig()
	if err != nil {
		sugaredTempLogger.Fatalf("failed to load configuration: %v", err)
	}

	rawLogger, err := newLogger(cfg.LogLevel)
	if err != nil {
		sugaredTempLogger.Fatalf("failed to create logger: %v", err)
	}
	zap.ReplaceGlobals(rawLogger)
	logger := rawLogger.Sugar()
	defer logger.Sync()

	if cfg.ECSCluster == "" || cfg.ECSService == "" {
		logger.Fatal("'-ecs-cluster' and '-ecs-service' are required")
	}

	snapshotCache := cache.NewSnapshotCache(false, cache.IDHash{}, logger)

	go runEcsPoller(context.Background(), snapshotCache, cfg.AWSRegion, cfg.ECSCluster, cfg.ECSService, cfg.NodeID, cfg.PollingInterval, logger)

	ctx := context.Background()
	cb := &test.Callbacks{}
	srv := server.NewServer(ctx, snapshotCache, cb)

	grpcServer := grpc.NewServer()
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.Port))
	if err != nil {
		logger.Fatalf("failed to listen: %v", err)
	}

	discoverygrpc.RegisterAggregatedDiscoveryServiceServer(grpcServer, srv)
	logger.Infof("management server listening on %d", cfg.Port)
	if err := grpcServer.Serve(lis); err != nil {
		logger.Errorw("failed to start grpc server", "error", err)
	}
}

func newLogger(level string) (*zap.Logger, error) {
	config := zap.NewProductionConfig()
	logLevel := zap.NewAtomicLevel()
	if err := logLevel.UnmarshalText([]byte(level)); err != nil {
		return nil, err
	}
	config.Level = logLevel
	return config.Build()
}
