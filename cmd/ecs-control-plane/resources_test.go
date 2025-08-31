package main

import (
	"testing"

	core "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	endpoint "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	"github.com/envoyproxy/go-control-plane/pkg/resource/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMakeCluster(t *testing.T) {
	cluster := makeCluster("test-cluster")
	assert.Equal(t, "test-cluster", cluster.Name)
	assert.Equal(t, core.ApiConfigSource_GRPC, cluster.EdsClusterConfig.EdsConfig.GetApiConfigSource().GetApiType())
}

func TestMakeRoute(t *testing.T) {
	routeCfg := makeRoute("test-route", "test-cluster")
	assert.Equal(t, "test-route", routeCfg.Name)
	require.Len(t, routeCfg.VirtualHosts, 1)
	require.Len(t, routeCfg.VirtualHosts[0].Routes, 1)
	assert.Equal(t, "test-cluster", routeCfg.VirtualHosts[0].Routes[0].GetRoute().GetCluster())
}

func TestMakeHTTPListener(t *testing.T) {
	listener := makeHTTPListener("test-listener", "test-route")
	assert.Equal(t, "test-listener", listener.Name)
	// A more thorough test could unmarshal the Any proto to check the HCM config.
}

func TestCreateSnapshot(t *testing.T) {
	endpoints := []*endpoint.LbEndpoint{
		{
			HostIdentifier: &endpoint.LbEndpoint_Endpoint{
				Endpoint: &endpoint.Endpoint{
					Address: &core.Address{
						Address: &core.Address_SocketAddress{
							SocketAddress: &core.SocketAddress{
								Address:       "10.0.0.1",
								PortSpecifier: &core.SocketAddress_PortValue{PortValue: 8080},
							},
						},
					},
				},
			},
		},
	}

	snapshot := createSnapshot("test-cluster", endpoints)
	require.NotNil(t, snapshot)

	// Verify consistency
	err := snapshot.Consistent()
	assert.NoError(t, err)

	// Verify resources
	clusters := snapshot.GetResources(resource.ClusterType)
	assert.Len(t, clusters, 1)
	assert.Contains(t, clusters, "test-cluster")

	routes := snapshot.GetResources(resource.RouteType)
	assert.Len(t, routes, 1)
	assert.Contains(t, routes, "local_route")

	listeners := snapshot.GetResources(resource.ListenerType)
	assert.Len(t, listeners, 1)
	assert.Contains(t, listeners, "listener_0")

	eds := snapshot.GetResources(resource.EndpointType)
	require.Len(t, eds, 1)
	require.Contains(t, eds, "test-cluster")
	cla, ok := eds["test-cluster"].(*endpoint.ClusterLoadAssignment)
	require.True(t, ok)
	require.Len(t, cla.Endpoints, 1)
	assert.Len(t, cla.Endpoints[0].LbEndpoints, 1)
}

func TestCreateEmptySnapshot(t *testing.T) {
	snapshot := createSnapshot("test-cluster", []*endpoint.LbEndpoint{})
	require.NotNil(t, snapshot)

	err := snapshot.Consistent()
	assert.NoError(t, err)

	eds := snapshot.GetResources(resource.EndpointType)
	require.Len(t, eds, 1)
	cla := eds["test-cluster"].(*endpoint.ClusterLoadAssignment)
	assert.Len(t, cla.Endpoints, 0)
}
