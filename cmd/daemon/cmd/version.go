package cmd

import (
    "fmt"

    "github.com/spf13/cobra"
)

var VersionCmd = &cobra.Command{
    Use:   "version",
    Short: "Show version",
    Run:   runVersion,
}

func runVersion(cmd *cobra.Command, args []string) {
    fmt.Println("Loom Daemon v0.1.0")
}
