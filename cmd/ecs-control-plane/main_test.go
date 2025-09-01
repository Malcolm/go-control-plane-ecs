package main

import (
	"os"
	"testing"
	"time"

	pflag "github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestLoadConfig(t *testing.T) {
	t.Run("defaults", func(t *testing.T) {
		flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
		cfg, err := loadConfig(zap.NewNop(), flags)
		require.NoError(t, err)
		assert.Equal(t, uint(18000), cfg.Port)
		assert.Equal(t, 10*time.Second, cfg.PollingInterval)
	})

	t.Run("env vars", func(t *testing.T) {
		os.Setenv("XDS_PORT", "12345")
		os.Setenv("XDS_POLLING_INTERVAL", "30s")
		defer os.Unsetenv("XDS_PORT")
		defer os.Unsetenv("XDS_POLLING_INTERVAL")

		flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
		cfg, err := loadConfig(zap.NewNop(), flags)
		require.NoError(t, err)
		assert.Equal(t, uint(12345), cfg.Port)
		assert.Equal(t, 30*time.Second, cfg.PollingInterval)
	})

	t.Run("flags override env vars", func(t *testing.T) {
		os.Setenv("XDS_PORT", "12345")
		defer os.Unsetenv("XDS_PORT")

		flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
		flags.Uint("port", 0, "") // Define the flag

		// Simulate the command line argument
		err := flags.Parse([]string{"--port=9999"})
		require.NoError(t, err)

		cfg, err := loadConfig(zap.NewNop(), flags)
		require.NoError(t, err)
		assert.Equal(t, uint(9999), cfg.Port)
	})
}
