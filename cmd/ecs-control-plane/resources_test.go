package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCreateSnapshot(t *testing.T) {
	snapshot := createSnapshot("v1", "test-cluster", nil)
	assert.NotNil(t, snapshot)
	assert.NoError(t, snapshot.Consistent())
}
