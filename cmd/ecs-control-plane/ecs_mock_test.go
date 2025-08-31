package main

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/ecs"
)

// mockEcsClient is a mock implementation of the ecsClient interface for use in tests.
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
