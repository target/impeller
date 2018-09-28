package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/target/impeller/types"
)

func TestOverridesValueFiles(t *testing.T) {
	p := &Plugin{
		ValueFiles: []string{"test", "file"},
	}
	release := &types.Release{}

	overrides := p.overrides(release)
	require.Len(t, overrides, 2)
	assert.Equal(t, "test", overrides[0].Value)
	assert.False(t, overrides[0].ValueSecret)
	assert.Equal(t, "file", overrides[1].Value)
	assert.False(t, overrides[1].ValueSecret)
}

func TestOverridesIndividualOverrides(t *testing.T) {
	override := "test"
	p := &Plugin{}
	release := &types.Release{
		Overrides: []types.Override{
			types.Override{
				Target: "image.tag",
				Value: types.Value{
					Value: &override,
				},
			},
		},
	}

	overrides := p.overrides(release)
	require.Len(t, overrides, 1)
	assert.Equal(t, "set", overrides[0].Name)
	assert.Equal(t, "image.tag=test", overrides[0].Value)
	assert.True(t, overrides[0].ValueSecret)
}
