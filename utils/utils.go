package utils

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strings"

	"github.com/target/impeller/types"
	"github.com/target/impeller/utils/report"

	"gopkg.in/yaml.v2"
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
// ListClusters function lists cluster configuration files
func ListClusters(configPath string) (cl report.Clusters, err error) {
	cl = report.NewClusters()
	dirList, err := ioutil.ReadDir(configPath)
	if err != nil {
		err = fmt.Errorf("Error opening file \"%s\": %v", configPath, err)
		return
	}
	for _, file := range dirList {
		if ! file.IsDir() {
			cl.Add(file.Name())
		}
	}


	return
}

func  GetValueFiles(valueFiles *[]string) (reportOverrides string) {
	if len(*valueFiles) >0 {
	for _,vf := range *valueFiles {
		reportOverrides = reportOverrides +" |"  + vf
	}
	} else {
		reportOverrides = "no overrides"
	}
	return
}
