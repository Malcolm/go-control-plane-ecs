package main

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfig(t *testing.T) {
	// Since pflag is used, we can't run these tests in parallel or in sub-tests
	// that both call loadConfig. This test function will be run once.

	// Test defaults
	cfg, err := loadConfig(simpleLogger{})
	require.NoError(t, err)
	assert.Equal(t, uint(18000), cfg.Port)
	assert.Equal(t, 10*time.Second, cfg.PollingInterval)

	// Test env vars
	os.Setenv("XDS_PORT", "12345")
	os.Setenv("XDS_POLLING_INTERVAL", "30s")
	defer os.Unsetenv("XDS_PORT")
	defer os.Unsetenv("XDS_POLLING_INTERVAL")

	// Re-load config to pick up env vars
	// This shows a limitation of this test setup; a new viper instance is needed
	// but flags are global. For this test, we assume viper picks up env vars
	// after initial load, which it does.
	cfg, err = loadConfig(simpleLogger{})
	require.NoError(t, err)
	assert.Equal(t, uint(12345), cfg.Port)
	assert.Equal(t, 30*time.Second, cfg.PollingInterval)
}
