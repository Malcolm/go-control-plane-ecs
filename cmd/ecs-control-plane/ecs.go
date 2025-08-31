package main

import (
	"context"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	core "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	endpoint "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	"github.com/envoyproxy/go-control-plane/pkg/cache/v3"
)

func runEcsPoller(ctx context.Context, snapshotCache cache.SnapshotCache, awsRegion, ecsClusterName, ecsServiceName, nodeID string, pollingInterval time.Duration, l Logger) {
	ticker := time.NewTicker(pollingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			updateEndpoints(ctx, snapshotCache, awsRegion, ecsClusterName, ecsServiceName, nodeID, l)
		case <-ctx.Done():
			return
		}
	}
}

func updateEndpoints(ctx context.Context, snapshotCache cache.SnapshotCache, awsRegion, ecsClusterName, ecsServiceName, nodeID string, l Logger) {
	l.Infof("Fetching ECS data for cluster %s, service %s", ecsClusterName, ecsServiceName)

	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(awsRegion))
	if err != nil {
		l.Errorf("failed to load aws config: %v", err)
		return
	}

	ecsClient := ecs.NewFromConfig(cfg)

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

	// Create a snapshot with the new endpoints.
	// We need to create the other resources as well.
	// For now, we'll just create the endpoint resource.
	// We will create the other resources in the resources.go file.
	snapshot := createSnapshot(ecsServiceName, endpoints)
	if err := snapshotCache.SetSnapshot(ctx, nodeID, snapshot); err != nil {
		l.Errorf("failed to set snapshot: %v", err)
	}
}
