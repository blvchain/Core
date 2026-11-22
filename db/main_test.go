package db

import (
	"io"
	"os"
	"testing"

	"blvchain/core/config"
	"blvchain/core/logger"
)

func TestMain(m *testing.M) {
	logger.SetOutput(io.Discard)

	config.SetConfigDir("../config")
	config.ReloadDotEnv()
	config.DELIUM_CONFIG = config.GetDeliumConfigFile()
	config.DNS_SEED_LIST = config.GetDnsSeedListFile()
	config.SELF_UID = "node_1_test"
	config.FIRST_BLOCK_HASH = ""
	config.MAX_DATA_SIZE_KB = "2048"

	ResetCollection()

	code := m.Run()
	os.Exit(code)
}
