package utils

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"

	"github.com/target/helm-propeller/constants"
	"github.com/target/helm-propeller/types"
	yaml "gopkg.in/yaml.v2"
)

// Run executes a provided command while also sending its output to stdout and
// stderr respectively. It allows printing the executed command and arguments,
// but this can be disabled if the command contains secrets.
func Run(cmd *exec.Cmd, showCommand bool) error {
	if showCommand {
		log.Printf("RUNNING: %s", strings.Join(cmd.Args, " "))
	} else {
		log.Printf("RUNNING COMMAND: (command hidden)")
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func ReadClusterConfig(configPath string) (config types.ClusterConfig, err error) {
	file, err := os.Open(configPath)
	if err != nil {
		err = fmt.Errorf("Error opening file \"%s\": %v", configPath, err)
		return
	}

	decoder := yaml.NewDecoder(file)
	err = decoder.Decode(&config)
	if err != nil {
		err = fmt.Errorf("Error decoding config file: %v", err)
		return
	}

	return
}

// PollTiller makes `retries` attempts to connect with the Tiller pod running
// in the cluster. A `tillerNamespace` can be set to override the default.
func PollTiller(retries int, tillerNamespace string) error {
	args := []string{"version"}
	if tillerNamespace != "" {
		args = append(args, "--tiller-namespace", tillerNamespace)
	}

	var err error
	for i := 0; i < retries; i++ {
		if err = Run(exec.Command(constants.HelmBin, args...), true); err == nil {
			return nil
		}
	}
	return err
}
