package main

import (
	"os"
	"testing"
	"time"

	pflag "github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfig(t *testing.T) {
	t.Run("defaults", func(t *testing.T) {
		pflag.CommandLine = pflag.NewFlagSet(os.Args[0], pflag.ExitOnError) // reset flags
		cfg, err := loadConfig()
		require.NoError(t, err)
		assert.Equal(t, uint(18000), cfg.Port)
		assert.Equal(t, 10*time.Second, cfg.PollingInterval)
	})

	t.Run("env vars", func(t *testing.T) {
		pflag.CommandLine = pflag.NewFlagSet(os.Args[0], pflag.ExitOnError) // reset flags
		os.Setenv("XDS_PORT", "12345")
		os.Setenv("XDS_POLLING_INTERVAL", "30s")
		defer os.Unsetenv("XDS_PORT")
		defer os.Unsetenv("XDS_POLLING_INTERVAL")

		cfg, err := loadConfig()
		require.NoError(t, err)
		assert.Equal(t, uint(12345), cfg.Port)
		assert.Equal(t, 30*time.Second, cfg.PollingInterval)
	})
}
