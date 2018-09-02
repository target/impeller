package utils

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadConfigWithRepoCredentials(t *testing.T) {
	config, err := ReadClusterConfig("./tests/sample_config_repo_credentials.yaml")
	require.Nil(t, err)

	username, err := config.Helm.Repos[0].Username.GetValue()
	require.Nil(t, err)
	assert.Equal(t, "myuser", username)

	os.Setenv("unittest-password", "testpassword")
	password, err := config.Helm.Repos[0].Password.GetValue()
	require.Nil(t, err)
	assert.Equal(t, "testpassword", password)
}

func TestReadConfigWithChartOverrides(t *testing.T) {
	config, err := ReadClusterConfig("./tests/sample_config_overrides.yaml")
	require.Nil(t, err)

	overrides := config.Releases[0].Overrides
	overrideValue, err := overrides[0].GetValue()
	require.Nil(t, err)
	assert.Equal(t, "unittest-value-1", overrideValue)

	overrideValue, err = overrides[1].GetValue()
	require.Nil(t, err)
	assert.Equal(t, "unittest-value-2", overrideValue)
}
