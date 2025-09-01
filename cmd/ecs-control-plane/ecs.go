package main

import (
	"context"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/retry"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	core "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	endpoint "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	"github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	"go.uber.org/zap"
)

// ecsClient defines the interface for the ECS API calls we need.
// This allows us to mock the client in tests.
type ecsClient interface {
	ListTasks(ctx context.Context, params *ecs.ListTasksInput, optFns ...func(*ecs.Options)) (*ecs.ListTasksOutput, error)
	DescribeTasks(ctx context.Context, params *ecs.DescribeTasksInput, optFns ...func(*ecs.Options)) (*ecs.DescribeTasksOutput, error)
}

func runEcsPoller(ctx context.Context, snapshotCache cache.SnapshotCache, awsRegion, ecsClusterName, ecsServiceName, nodeID string, pollingInterval time.Duration, logger *zap.Logger) {
	logger.Info("starting ECS poller with custom retryer")
	customRetryer := retry.NewStandard(func(o *retry.StandardOptions) {
		o.MaxAttempts = 10
		o.MaxBackoff = 10 * time.Second
	})

	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(awsRegion),
		config.WithRetryer(func() aws.Retryer { return customRetryer }),
	)
	if err != nil {
		logger.Error("runEcsPoller: failed to load aws config", zap.Error(err))
		return
	}
	ecsClient := ecs.NewFromConfig(cfg)

	ticker := time.NewTicker(pollingInterval)
	defer ticker.Stop()

	// Initial fetch
	updateEndpoints(ctx, ecsClient, snapshotCache, ecsClusterName, ecsServiceName, nodeID, logger)

	for {
		select {
		case <-ticker.C:
			updateEndpoints(ctx, ecsClient, snapshotCache, ecsClusterName, ecsServiceName, nodeID, logger)
		case <-ctx.Done():
			return
		}
	}
}

func updateEndpoints(ctx context.Context, ecsClient ecsClient, snapshotCache cache.SnapshotCache, ecsClusterName, ecsServiceName, nodeID string, logger *zap.Logger) {
	logger.Info("fetching ECS data", zap.String("cluster", ecsClusterName), zap.String("service", ecsServiceName))

	listTasksOutput, err := ecsClient.ListTasks(ctx, &ecs.ListTasksInput{
		Cluster:     &ecsClusterName,
		ServiceName: &ecsServiceName,
	})
	if err != nil {
		logger.Error("failed to list ecs tasks", zap.Error(err))
		awsAPIErrors.Inc()
		return
	}

	if len(listTasksOutput.TaskArns) == 0 {
		logger.Info("no tasks found for service, clearing endpoints", zap.String("service", ecsServiceName))
		snapshot := createSnapshot(ecsServiceName, []*endpoint.LbEndpoint{})
		if err := snapshotCache.SetSnapshot(ctx, nodeID, snapshot); err != nil {
			logger.Error("failed to set snapshot", zap.Error(err))
		} else {
			snapshotUpdates.Inc()
		}
		endpointsDiscovered.Set(0)
		return
	}

	describeTasksOutput, err := ecsClient.DescribeTasks(ctx, &ecs.DescribeTasksInput{
		Cluster: &ecsClusterName,
		Tasks:   listTasksOutput.TaskArns,
	})
	if err != nil {
		logger.Error("failed to describe ecs tasks", zap.Error(err))
		awsAPIErrors.Inc()
		return
	}

	var endpoints []*endpoint.LbEndpoint
	for _, task := range describeTasksOutput.Tasks {
		var ipAddress string
		for _, attachment := range task.Attachments {
			if *attachment.Type == "ElasticNetworkInterface" {
				for _, detail := range attachment.Details {
					if *detail.Name == "privateIPv4Address" {
						ipAddress = *detail.Value
						break
					}
				}
			}
			if ipAddress != "" {
				break
			}
		}

		if ipAddress == "" {
			logger.Warn("could not find private IP for task", zap.String("task_arn", *task.TaskArn))
			continue
		}

		for _, container := range task.Containers {
			for _, portMapping := range container.NetworkBindings {
				lbEndpoint := &endpoint.LbEndpoint{
					HostIdentifier: &endpoint.LbEndpoint_Endpoint{
						Endpoint: &endpoint.Endpoint{
							Address: &core.Address{
								Address: &core.Address_SocketAddress{
									SocketAddress: &core.SocketAddress{
										Protocol:      core.SocketAddress_TCP,
										Address:       ipAddress,
										PortSpecifier: &core.SocketAddress_PortValue{PortValue: uint32(*portMapping.HostPort)},
									},
								},
							},
						},
					},
				}
				endpoints = append(endpoints, lbEndpoint)
			}
		}
	}

	logger.Info("found endpoints for service", zap.Int("count", len(endpoints)), zap.String("service", ecsServiceName))
	endpointsDiscovered.Set(float64(len(endpoints)))

	snapshot := createSnapshot(ecsServiceName, endpoints)
	if err := snapshotCache.SetSnapshot(ctx, nodeID, snapshot); err != nil {
		logger.Error("failed to set snapshot", zap.Error(err))
	} else {
		snapshotUpdates.Inc()
	}
}
