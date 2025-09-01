package main

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	endpoint "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	"github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	"github.com/envoyproxy/go-control-plane/pkg/resource/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestUpdateEndpoints(t *testing.T) {
	// Helper function to create a string pointer
	s := func(str string) *string { return &str }
	// Helper function to create an int32 pointer
	i := func(i int32) *int32 { return &i }

	testCases := []struct {
		name                 string
		mockListTasks        *ecs.ListTasksOutput
		mockListTasksErr     error
		mockDescribeTasks    *ecs.DescribeTasksOutput
		mockDescribeTasksErr error
		expectedEndpoints    int
		expectSnapshot       bool
	}{
		{
			name: "success with two tasks",
			mockListTasks: &ecs.ListTasksOutput{
				TaskArns: []string{"arn:1", "arn:2"},
			},
			mockDescribeTasks: &ecs.DescribeTasksOutput{
				Tasks: []types.Task{
					{
						TaskArn: s("arn:1"),
						Attachments: []types.Attachment{{
							Type:    s("ElasticNetworkInterface"),
							Details: []types.KeyValuePair{{Name: s("privateIPv4Address"), Value: s("10.0.0.1")}},
						}},
						Containers: []types.Container{{
							NetworkBindings: []types.NetworkBinding{{HostPort: i(8080)}},
						}},
					},
					{
						TaskArn: s("arn:2"),
						Attachments: []types.Attachment{{
							Type:    s("ElasticNetworkInterface"),
							Details: []types.KeyValuePair{{Name: s("privateIPv4Address"), Value: s("10.0.0.2")}},
						}},
						Containers: []types.Container{{
							NetworkBindings: []types.NetworkBinding{{HostPort: i(8081)}},
						}},
					},
				},
			},
			expectedEndpoints: 2,
			expectSnapshot:    true,
		},
		{
			name:              "no tasks found",
			mockListTasks:     &ecs.ListTasksOutput{TaskArns: []string{}},
			expectedEndpoints: 0,
			expectSnapshot:    true,
		},
		{
			name:             "list tasks returns error",
			mockListTasksErr: errors.New("list tasks error"),
			expectSnapshot:   false,
		},
		{
			name: "describe tasks returns error",
			mockListTasks: &ecs.ListTasksOutput{
				TaskArns: []string{"arn:1"},
			},
			mockDescribeTasksErr: errors.New("describe tasks error"),
			expectSnapshot:       false,
		},
		{
			name: "task with no private ip",
			mockListTasks: &ecs.ListTasksOutput{
				TaskArns: []string{"arn:1"},
			},
			mockDescribeTasks: &ecs.DescribeTasksOutput{
				Tasks: []types.Task{
					{
						TaskArn: s("arn:1"),
						Attachments: []types.Attachment{{
							Type:    s("ElasticNetworkInterface"),
							Details: []types.KeyValuePair{{Name: s("other"), Value: s("value")}},
						}},
					},
				},
			},
			expectedEndpoints: 0,
			expectSnapshot:    true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockClient := &mockEcsClient{
				ListTasksFunc: func(ctx context.Context, params *ecs.ListTasksInput, optFns ...func(*ecs.Options)) (*ecs.ListTasksOutput, error) {
					return tc.mockListTasks, tc.mockListTasksErr
				},
				DescribeTasksFunc: func(ctx context.Context, params *ecs.DescribeTasksInput, optFns ...func(*ecs.Options)) (*ecs.DescribeTasksOutput, error) {
					return tc.mockDescribeTasks, tc.mockDescribeTasksErr
				},
			}

			snapshotCache := cache.NewSnapshotCache(false, cache.IDHash{}, nil)

			updateEndpoints(context.Background(), mockClient, snapshotCache, "test-cluster", "test-service", "test-node", zap.NewNop())

			snapshot, err := snapshotCache.GetSnapshot("test-node")
			if tc.expectSnapshot {
				require.NoError(t, err)
				endpointsResource := snapshot.GetResources(resource.EndpointType)
				require.Contains(t, endpointsResource, "test-service")
				cla, ok := endpointsResource["test-service"].(*endpoint.ClusterLoadAssignment)
				require.True(t, ok)
				if tc.expectedEndpoints > 0 {
					require.Len(t, cla.Endpoints, 1)
					assert.Len(t, cla.Endpoints[0].LbEndpoints, tc.expectedEndpoints)
				} else {
					assert.Len(t, cla.Endpoints, 0)
				}
			} else {
				assert.Error(t, err, "expected no snapshot to be set")
			}
		})
	}
}
