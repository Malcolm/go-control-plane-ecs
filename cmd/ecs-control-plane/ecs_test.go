package main

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	"github.com/stretchr/testify/assert"
)

type mockEcsClient struct {
	ListTasksFunc     func(ctx context.Context, params *ecs.ListTasksInput, optFns ...func(*ecs.Options)) (*ecs.ListTasksOutput, error)
	DescribeTasksFunc func(ctx context.Context, params *ecs.DescribeTasksInput, optFns ...func(*ecs.Options)) (*ecs.DescribeTasksOutput, error)
}

func (m *mockEcsClient) ListTasks(ctx context.Context, params *ecs.ListTasksInput, optFns ...func(*ecs.Options)) (*ecs.ListTasksOutput, error) {
	return m.ListTasksFunc(ctx, params, optFns...)
}

func (m *mockEcsClient) DescribeTasks(ctx context.Context, params *ecs.DescribeTasksInput, optFns ...func(*ecs.Options)) (*ecs.DescribeTasksOutput, error) {
	return m.DescribeTasksFunc(ctx, params, optFns...)
}

func TestUpdateEndpoints(t *testing.T) {
	s := func(str string) *string { return &str }
	i := func(i int32) *int32 { return &i }

	mockClient := &mockEcsClient{
		ListTasksFunc: func(ctx context.Context, params *ecs.ListTasksInput, optFns ...func(*ecs.Options)) (*ecs.ListTasksOutput, error) {
			return &ecs.ListTasksOutput{TaskArns: []string{"arn:1"}}, nil
		},
		DescribeTasksFunc: func(ctx context.Context, params *ecs.DescribeTasksInput, optFns ...func(*ecs.Options)) (*ecs.DescribeTasksOutput, error) {
			return &ecs.DescribeTasksOutput{Tasks: []types.Task{{
				TaskArn:     s("arn:1"),
				Attachments: []types.Attachment{{Type: s("ElasticNetworkInterface"), Details: []types.KeyValuePair{{Name: s("privateIPv4Address"), Value: s("10.0.0.1")}}}},
				Containers:  []types.Container{{NetworkBindings: []types.NetworkBinding{{HostPort: i(8080)}}}},
			}}}, nil
		},
	}

	snapshotCache := cache.NewSnapshotCache(false, cache.IDHash{}, simpleLogger{})
	updateEndpoints(context.Background(), mockClient, snapshotCache, "cluster", "service", "node", simpleLogger{})

	snapshot, err := snapshotCache.GetSnapshot("node")
	assert.NoError(t, err)
	assert.NotNil(t, snapshot)
}

func TestUpdateEndpoints_ListTasksError(t *testing.T) {
	mockClient := &mockEcsClient{
		ListTasksFunc: func(ctx context.Context, params *ecs.ListTasksInput, optFns ...func(*ecs.Options)) (*ecs.ListTasksOutput, error) {
			return nil, errors.New("aws error")
		},
	}

	snapshotCache := cache.NewSnapshotCache(false, cache.IDHash{}, simpleLogger{})
	updateEndpoints(context.Background(), mockClient, snapshotCache, "cluster", "service", "node", simpleLogger{})

	_, err := snapshotCache.GetSnapshot("node")
	assert.Error(t, err)
}
