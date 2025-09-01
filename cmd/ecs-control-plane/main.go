package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	"github.com/envoyproxy/go-control-plane/pkg/server/v3"
	"github.com/envoyproxy/go-control-plane/pkg/test/v3"
	discoverygrpc "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	pflag "github.com/spf13/pflag"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

// Config holds the application's configuration.
type Config struct {
	Debug           bool          `mapstructure:"debug"`
	Port            uint          `mapstructure:"port"`
	MetricsPort     uint          `mapstructure:"metrics-port"`
	NodeID          string        `mapstructure:"node-id"`
	AWSRegion       string        `mapstructure:"aws-region"`
	ECSCluster      string        `mapstructure:"ecs-cluster"`
	ECSService      string        `mapstructure:"ecs-service"`
	PollingInterval time.Duration `mapstructure:"polling-interval"`
}

// zapCacheLogger is an adapter to make zap.Logger compatible with cache.Logger.
type zapCacheLogger struct {
	logger *zap.Logger
}

func (l zapCacheLogger) Debugf(format string, args ...interface{}) {
	l.logger.Debug(fmt.Sprintf(format, args...))
}
func (l zapCacheLogger) Infof(format string, args ...interface{}) {
	l.logger.Info(fmt.Sprintf(format, args...))
}
func (l zapCacheLogger) Warnf(format string, args ...interface{}) {
	l.logger.Warn(fmt.Sprintf(format, args...))
}
func (l zapCacheLogger) Errorf(format string, args ...interface{}) {
	l.logger.Error(fmt.Sprintf(format, args...))
}

// loadConfig loads configuration from file, environment variables, and flags.
func loadConfig(logger *zap.Logger, flags *pflag.FlagSet) (*Config, error) {
	v := viper.New()

	// Set defaults
	v.SetDefault("debug", false)
	v.SetDefault("port", 18000)
	v.SetDefault("metrics-port", 9090)
	v.SetDefault("node-id", "test-id")
	v.SetDefault("polling-interval", 10*time.Second)

	// Bind command-line flags
	v.BindPFlags(flags)

	// Bind environment variables
	v.SetEnvPrefix("XDS")
	v.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	v.AutomaticEnv()

	// Read from config file
	v.SetConfigName("config")
	v.AddConfigPath(".")
	v.AddConfigPath("/etc/ecs-control-plane/")
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			logger.Debug("config file not found, using flags and environment variables")
		} else {
			return nil, err
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// RunServer starts an xDS server at the given port.
func RunServer(ctx context.Context, srv server.Server, port uint, logger *zap.Logger) {
	grpcServer := grpc.NewServer()
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		logger.Fatal("failed to listen", zap.Error(err))
	}

	discoverygrpc.RegisterAggregatedDiscoveryServiceServer(grpcServer, srv)

	go func() {
		logger.Info("management server listening", zap.Uint("port", port))
		if err := grpcServer.Serve(lis); err != nil {
			logger.Error("management server failed", zap.Error(err))
		}
	}()

	<-ctx.Done()
	logger.Info("shutting down management server")
	grpcServer.GracefulStop()
}

func runMetricsServer(ctx context.Context, port uint, logger *zap.Logger) {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "OK")
	})
	server := &http.Server{Addr: fmt.Sprintf(":%d", port), Handler: mux}

	go func() {
		logger.Info("metrics server listening", zap.Uint("port", port))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("metrics server failed", zap.Error(err))
		}
	}()

	<-ctx.Done()
	logger.Info("shutting down metrics server")
	if err := server.Shutdown(context.Background()); err != nil {
		logger.Error("metrics server shutdown failed", zap.Error(err))
	}
}

func main() {
	// Define flags
	pflag.Bool("debug", false, "Enable xDS server debug logging")
	pflag.Uint("port", 18000, "xDS management server port")
	pflag.Uint("metrics-port", 9090, "Metrics server port")
	pflag.String("node-id", "test-id", "Node ID")
	pflag.String("aws-region", "us-west-2", "AWS region")
	pflag.String("ecs-cluster", "", "ECS cluster name")
	pflag.String("ecs-service", "", "ECS service name")
	pflag.Duration("polling-interval", 10*time.Second, "Polling interval for ECS metadata")
	pflag.Parse()

	// A temporary logger for config loading
	tmpLogger, _ := zap.NewProduction()
	cfg, err := loadConfig(tmpLogger, pflag.CommandLine)
	if err != nil {
		tmpLogger.Fatal("failed to load configuration", zap.Error(err))
	}

	var logger *zap.Logger
	if cfg.Debug {
		logger, err = zap.NewDevelopment()
	} else {
		logger, err = zap.NewProduction()
	}
	if err != nil {
		log.Fatalf("failed to create logger: %v", err)
	}
	defer logger.Sync()

	if cfg.ECSCluster == "" {
		logger.Fatal("ECS cluster name is required")
	}
	if cfg.ECSService == "" {
		logger.Fatal("ECS service name is required")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Start the metrics server
	go runMetricsServer(ctx, cfg.MetricsPort, logger)

	// Create a cache
	cacheLogger := &zapCacheLogger{logger: logger}
	cache := cache.NewSnapshotCache(false, cache.IDHash{}, cacheLogger)

	// Start the ECS poller goroutine
	go runEcsPoller(ctx, cache, cfg.AWSRegion, cfg.ECSCluster, cfg.ECSService, cfg.NodeID, cfg.PollingInterval, logger)

	// Run the xDS server
	cb := &test.Callbacks{Debug: cfg.Debug}
	srv := server.NewServer(ctx, cache, cb)
	go RunServer(ctx, srv, cfg.Port, logger)

	<-ctx.Done()
	logger.Info("shutting down")
}
