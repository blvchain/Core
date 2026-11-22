package main

import (
	"io"
	"os"
	"testing"

	"blvchain/core/config"
	"blvchain/core/logger"
)

func TestMain(m *testing.M) {
	logger.SetOutput(io.Discard)
	os.Exit(m.Run())
}

// run() validates the configuration before it touches MongoDB, so an invalid
// configuration must fail fast and without any network access.
func TestRunFailsFastOnInvalidConfiguration(t *testing.T) {
	original := config.SELF_UID
	config.SELF_UID = ""
	t.Cleanup(func() { config.SELF_UID = original })

	if err := run(); err == nil {
		t.Fatal("run() accepted a configuration without SELF_UID")
	}
}

func TestRunFailsFastOnMissingMongoURI(t *testing.T) {
	original := config.MONGO_URI
	config.MONGO_URI = ""
	t.Cleanup(func() { config.MONGO_URI = original })

	if err := run(); err == nil {
		t.Fatal("run() accepted a configuration without MONGO_URI")
	}
}
