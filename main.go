package main

import (
	"fmt"
	"log"
	"os"

	"github.com/target/propeller/utils"
	"github.com/urfave/cli"
)

func main() {
	app := cli.NewApp()
	app.Name = "addon-manager"
	app.Action = run
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:   "cluster-config-path",
			Usage:  "Path to the cluster config",
			EnvVar: "CLUSTER_CONFIG,PLUGIN_CLUSTER_CONFIG",
		},
		cli.StringSliceFlag{
			Name:   "value-files",
			Usage:  "Helm value override files",
			EnvVar: "VALUE_FILES,PLUGIN_VALUE_FILES",
		},
		cli.StringFlag{
			Name:   "kube-config",
			Usage:  "Kubernetes configuration file",
			EnvVar: "KUBE_CONFIG,PLUGIN_KUBE_CONFIG",
		},
		cli.StringFlag{
			Name:   "kube-context",
			Usage:  "Kubernetes configuration context to use",
			EnvVar: "KUBE_CONTEXT,PLUGIN_KUBE_CONTEXT",
		},
		cli.BoolFlag{
			Name:   "dry-run",
			Usage:  "Enables a dry_run deployment",
			EnvVar: "DRY_RUN,PLUGIN_DRY_RUN",
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}

func run(ctx *cli.Context) error {
	if ctx.String("cluster-config-path") == "" {
		return fmt.Errorf("Cluster config path not set.")
	}
	if ctx.String("kube-config") == "" {
		return fmt.Errorf("Kube config not set.")
	}
	if ctx.String("kube-context") == "" {
		return fmt.Errorf("Kube context not set.")
	}

	clusterConfig, err := utils.ReadClusterConfig(ctx.String("cluster-config-path"))
	if err != nil {
		return fmt.Errorf("Error reading cluster config: %v", err)
	}

	plugin := Plugin{
		ClusterConfig: clusterConfig,
		ValueFiles:    ctx.StringSlice("value-files"),
		KubeConfig:    ctx.String("kube-config"),
		KubeContext:   ctx.String("kube-context"),
		Dryrun:        ctx.Bool("dry-run"),
	}

	return plugin.Exec()
}
