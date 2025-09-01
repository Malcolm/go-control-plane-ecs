package main

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestMetricsAndHealthServer(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start the server in a goroutine
	go runMetricsServer(ctx, 9999, zap.NewNop())

	// Give the server a moment to start
	time.Sleep(100 * time.Millisecond)

	// Test health check
	t.Run("health check", func(t *testing.T) {
		resp, err := http.Get("http://localhost:9999/healthz")
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		assert.Equal(t, "OK", string(body))
	})

	// Test metrics
	t.Run("metrics", func(t *testing.T) {
		// Update a metric to ensure it's in the output
		snapshotUpdates.Inc()

		resp, err := http.Get("http://localhost:9999/metrics")
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		// Check for some of the metric names
		assert.True(t, strings.Contains(string(body), "xds_snapshot_updates_total"))
		assert.True(t, strings.Contains(string(body), "xds_endpoints_discovered"))
	})

	// Test shutdown
	cancel()
	// Give the server a moment to shut down
	time.Sleep(100 * time.Millisecond)
	_, err := http.Get("http://localhost:9999/healthz")
	assert.Error(t, err, "server should be shut down")
}
