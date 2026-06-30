package models

import (
	"os"
	"testing"

	"github.com/SeaCloudAI/seacloud-cli/internal/config"
)

func TestMain(m *testing.M) {
	home, err := os.MkdirTemp("", "seacloud-models-test-home-*")
	if err != nil {
		panic(err)
	}
	_ = os.Setenv("HOME", home)
	_ = os.Setenv("SEACLOUD_NO_KEYCHAIN", "1")
	_ = os.Setenv(config.EnvFolkosExecToken, "api-key")
	code := m.Run()
	_ = os.RemoveAll(home)
	os.Exit(code)
}
