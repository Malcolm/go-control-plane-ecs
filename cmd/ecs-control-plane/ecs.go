package main

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	core "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	endpoint "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	"github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	"go.uber.org/zap"
)

func runEcsPoller(ctx context.Context, snapshotCache cache.SnapshotCache, awsRegion, ecsCluster, ecsService, nodeID string, pollingInterval time.Duration, logger *zap.SugaredLogger) {
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(awsRegion))
	if err != nil {
		logger.Errorw("failed to load aws config", "error", err)
		return
	}
	ecsClient := ecs.NewFromConfig(cfg)

	ticker := time.NewTicker(pollingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			updateEndpoints(ctx, ecsClient, snapshotCache, ecsCluster, ecsService, nodeID, logger)
		}
	}
}

type ecsClient interface {
	ListTasks(ctx context.Context, params *ecs.ListTasksInput, optFns ...func(*ecs.Options)) (*ecs.ListTasksOutput, error)
	DescribeTasks(ctx context.Context, params *ecs.DescribeTasksInput, optFns ...func(*ecs.Options)) (*ecs.DescribeTasksOutput, error)
}

func updateEndpoints(ctx context.Context, ecsClient ecsClient, snapshotCache cache.SnapshotCache, ecsCluster, ecsService, nodeID string, logger *zap.SugaredLogger) {
	logger.Infow("fetching endpoints for service", "service", ecsService)

	listTasksOutput, err := ecsClient.ListTasks(ctx, &ecs.ListTasksInput{
		Cluster:     &ecsCluster,
		ServiceName: &ecsService,
	})
	if err != nil {
		logger.Errorw("failed to list ecs tasks", "error", err)
		return
	}

	var endpoints []*endpoint.LbEndpoint
	if len(listTasksOutput.TaskArns) > 0 {
		describeTasksOutput, err := ecsClient.DescribeTasks(ctx, &ecs.DescribeTasksInput{
			Cluster: &ecsCluster,
			Tasks:   listTasksOutput.TaskArns,
		})
		if err != nil {
			logger.Errorw("failed to describe ecs tasks", "error", err)
			return
		}
		endpoints = extractEndpoints(describeTasksOutput.Tasks)
	}

	logger.Infow("found endpoints for service", "count", len(endpoints), "service", ecsService)

	version := fmt.Sprintf("%d", time.Now().Unix())
	snapshot := createSnapshot(version, ecsService, endpoints)
	if err := snapshot.Consistent(); err != nil {
		logger.Errorw("new snapshot is not consistent", "error", err)
		return
	}

	if err := snapshotCache.SetSnapshot(ctx, nodeID, snapshot); err != nil {
		logger.Errorw("failed to set snapshot", "error", err)
	}
}

func extractEndpoints(tasks []types.Task) []*endpoint.LbEndpoint {
	var endpoints []*endpoint.LbEndpoint
	for _, task := range tasks {
		var ipAddress string
		for _, attachment := range task.Attachments {
			if attachment.Type != nil && *attachment.Type == "ElasticNetworkInterface" {
				for _, detail := range attachment.Details {
					if detail.Name != nil && *detail.Name == "privateIPv4Address" {
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
			continue
		}

		for _, container := range task.Containers {
			for _, portMapping := range container.NetworkBindings {
				ep := &endpoint.LbEndpoint{
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
				endpoints = append(endpoints, ep)
			}
		}
	}
	return endpoints
}
