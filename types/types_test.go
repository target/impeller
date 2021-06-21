package types

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/target/impeller/utils/commandbuilder"
)

func TestOverrideBuildArgDirectValue(t *testing.T) {
	value := "test"
	override := Override{
		Value: Value{
			Value:     &value,
			ShowValue: true,
		},
		Target: "value",
	}

	arg, err := override.BuildArg()
	require.NoError(t, err)
	assert.Equal(t, &commandbuilder.Arg{
		Type:        commandbuilder.ArgTypeLongParam,
		Name:        "set",
		Value:       "value=test",
		ValueSecret: false,
	}, arg)
}

func TestOverrideBuildArgEnvironment(t *testing.T) {
	os.Setenv("TEST_ENV", "test-value")
	override := Override{
		Value: Value{
			ValueFrom: &ValueFrom{
				Environment: "TEST_ENV",
			},
			ShowValue: false,
		},
		Target: "value",
	}

	arg, err := override.BuildArg()
	require.NoError(t, err)
	assert.Equal(t, &commandbuilder.Arg{
		Type:        commandbuilder.ArgTypeLongParam,
		Name:        "set",
		Value:       "value=test-value",
		ValueSecret: true,
	}, arg)
}

func TestOverrideBuildArgFile(t *testing.T) {
	override := Override{
		Value: Value{
			ValueFrom: &ValueFrom{
				File: "/tmp/fake-file",
			},
			ShowValue: true,
		},
		Target: "value",
	}

	arg, err := override.BuildArg()
	require.NoError(t, err)
	assert.Equal(t, &commandbuilder.Arg{
		Type:        commandbuilder.ArgTypeLongParam,
		Name:        "set-file",
		Value:       "value=/tmp/fake-file",
		ValueSecret: false,
	}, arg)
}
