package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	assert.Equal(t, "8084", cfg.Server.Port)
	assert.Equal(t, "tasks.dispatch", cfg.NATS.TaskDispatchSubject)
	assert.Equal(t, "tasks.results", cfg.NATS.TaskResultSubject)
	assert.Equal(t, 3, cfg.Orchestrator.MaxAttempts)
}

func TestLoadConfig_MissingFileReturnsDefaults(t *testing.T) {
	cfg, err := LoadConfig("does-not-exist.yaml")
	assert.NoError(t, err)
	assert.Equal(t, DefaultConfig(), cfg)
}

func TestValidate(t *testing.T) {
	cfg := DefaultConfig()
	assert.NoError(t, cfg.Validate())

	bad := DefaultConfig()
	bad.Orchestrator.MaxAttempts = 0
	assert.Error(t, bad.Validate())

	bad2 := DefaultConfig()
	bad2.NATS.URL = ""
	assert.Error(t, bad2.Validate())
}
