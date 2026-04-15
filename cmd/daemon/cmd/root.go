package cmd

import (
    "fmt"
    "github.com/spf13/cobra"
)

var Root = &cobra.Command{
    Use:   "loomd",
    Short: "Loom Daemon - Local AI Runtime Manager",
    Long:  "Loom Daemon connects local AI Runtime (Claude Code, OpenCode) to Loom Cloud",
}

func Execute() error {
    return Root.Execute()
}

var (
    verbose bool
    config  string
)

func init() {
    Root.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Verbose output")
    Root.PersistentFlags().StringVar(&config, "config", "config.yaml", "Config file path")
    Root.AddCommand(StartCmd)
    Root.AddCommand(VersionCmd)
}
