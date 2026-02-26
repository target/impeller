package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/target/impeller/types"
	"github.com/target/impeller/utils"
	"github.com/target/impeller/utils/report"
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

func TestOverridesIndividualOverridesPrint(t *testing.T) {
	override := "test"
	p := &Plugin{}
	release := &types.Release{
		Overrides: []types.Override{
			types.Override{
				Target: "image.tag",
				Value: types.Value{
					Value:     &override,
					ShowValue: true,
				},
			},
		},
	}

	overrides := p.overrides(release)
	require.Len(t, overrides, 1)
	assert.Equal(t, "set", overrides[0].Name)
	assert.Equal(t, "image.tag=test", overrides[0].Value)
	assert.False(t, overrides[0].ValueSecret)
}

func TestPlugin_ExecReport(t *testing.T) {
	p := Plugin{
		ClusterConfigPath: "./test-clusters",
		ClustersList:      report.Clusters{},
		ValueFiles:        nil,
		KubeConfig:        "",
		KubeContext:       "",
		Dryrun:            false,
		Diffrun:           false,
		Audit:             true,
		AuditFile:         "./go-test.csv",
	}
	clist, err := utils.ListClusters(p.ClusterConfigPath)
	require.Nil(t, err)
	p.ClustersList = clist
	err = p.Exec()
	require.Nil(t, err)

}

func TestGetYAMLFilesFromDir(t *testing.T) {
	tempDir := t.TempDir()

	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "a.yaml"), []byte("kind: ConfigMap"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "b.yml"), []byte("kind: Service"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "notes.txt"), []byte("ignore"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "kustomization.yaml"), []byte("resources: []"), 0o644))

	child := filepath.Join(tempDir, "subdir")
	require.NoError(t, os.Mkdir(child, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(child, "child.yaml"), []byte("kind: Secret"), 0o644))

	p := &Plugin{}
	files, err := p.getYAMLFilesFromDir(tempDir)
	require.NoError(t, err)

	assert.ElementsMatch(t, []string{
		filepath.Join(tempDir, "a.yaml"),
		filepath.Join(tempDir, "b.yml"),
	}, files)
}

func TestApplyKubectlFilesSkipsOnDryrun(t *testing.T) {
	p := &Plugin{Dryrun: true}
	release := &types.Release{KubectlFiles: []string{"does-not-need-to-exist.yaml"}}

	err := p.applyKubectlFiles(release)
	require.NoError(t, err)
}

func TestApplyKubectlFilesSkipsOnDiffrun(t *testing.T) {
	p := &Plugin{Diffrun: true}
	release := &types.Release{KubectlFiles: []string{"does-not-need-to-exist.yaml"}}

	err := p.applyKubectlFiles(release)
	require.NoError(t, err)
}

func TestWaitForResourcesSkipsOnDryrun(t *testing.T) {
	p := &Plugin{Dryrun: true}
	release := &types.Release{
		WaitforDeployment:  []string{"sample-deploy"},
		WaitforDaemonSet:   []string{"sample-daemon"},
		WaitforStatefulSet: []string{"sample-stateful"},
	}

	err := p.waitForResources(release)
	require.NoError(t, err)
}

func TestWaitForResourcesSkipsOnDiffrun(t *testing.T) {
	p := &Plugin{Diffrun: true}
	release := &types.Release{
		WaitforDeployment:  []string{"sample-deploy"},
		WaitforDaemonSet:   []string{"sample-daemon"},
		WaitforStatefulSet: []string{"sample-stateful"},
	}

	err := p.waitForResources(release)
	require.NoError(t, err)
}
