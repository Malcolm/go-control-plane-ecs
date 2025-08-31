package main

import (
	"context"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	core "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	endpoint "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	"github.com/envoyproxy/go-control-plane/pkg/cache/v3"
)

// ecsClient defines the interface for the ECS API calls we need.
// This allows us to mock the client in tests.
type ecsClient interface {
	ListTasks(ctx context.Context, params *ecs.ListTasksInput, optFns ...func(*ecs.Options)) (*ecs.ListTasksOutput, error)
	DescribeTasks(ctx context.Context, params *ecs.DescribeTasksInput, optFns ...func(*ecs.Options)) (*ecs.DescribeTasksOutput, error)
}

func runEcsPoller(ctx context.Context, snapshotCache cache.SnapshotCache, awsRegion, ecsClusterName, ecsServiceName, nodeID string, pollingInterval time.Duration, l Logger) {
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(awsRegion))
	if err != nil {
		l.Errorf("runEcsPoller: failed to load aws config: %v", err)
		return
	}
	ecsClient := ecs.NewFromConfig(cfg)

	ticker := time.NewTicker(pollingInterval)
	defer ticker.Stop()

	// Initial fetch
	updateEndpoints(ctx, ecsClient, snapshotCache, ecsClusterName, ecsServiceName, nodeID, l)

	for {
		select {
		case <-ticker.C:
			updateEndpoints(ctx, ecsClient, snapshotCache, ecsClusterName, ecsServiceName, nodeID, l)
		case <-ctx.Done():
			return
		}
	}
}

func updateEndpoints(ctx context.Context, ecsClient ecsClient, snapshotCache cache.SnapshotCache, ecsClusterName, ecsServiceName, nodeID string, l Logger) {
	l.Infof("Fetching ECS data for cluster %s, service %s", ecsClusterName, ecsServiceName)

	listTasksOutput, err := ecsClient.ListTasks(ctx, &ecs.ListTasksInput{
		Cluster:     &ecsClusterName,
		ServiceName: &ecsServiceName,
	})
	if err != nil {
		l.Errorf("failed to list ecs tasks: %v", err)
		return
	}

	if len(listTasksOutput.TaskArns) == 0 {
		l.Infof("no tasks found for service %s, clearing endpoints", ecsServiceName)
		snapshot := createSnapshot(ecsServiceName, []*endpoint.LbEndpoint{})
		if err := snapshotCache.SetSnapshot(ctx, nodeID, snapshot); err != nil {
			l.Errorf("failed to set snapshot: %v", err)
		}
		return
	}

	describeTasksOutput, err := ecsClient.DescribeTasks(ctx, &ecs.DescribeTasksInput{
		Cluster: &ecsClusterName,
		Tasks:   listTasksOutput.TaskArns,
	})
	if err != nil {
		l.Errorf("failed to describe ecs tasks: %v", err)
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
			l.Warnf("could not find private IP for task %s", *task.TaskArn)
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

	l.Infof("found %d endpoints for service %s", len(endpoints), ecsServiceName)

	snapshot := createSnapshot(ecsServiceName, endpoints)
	if err := snapshotCache.SetSnapshot(ctx, nodeID, snapshot); err != nil {
		l.Errorf("failed to set snapshot: %v", err)
	}
}
