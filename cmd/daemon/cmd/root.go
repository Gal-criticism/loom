package cmd

import (
	"github.com/spf13/cobra"
)

var (
	// Version is the daemon version
	Version = "0.1.0"

	// Root is the root command
	Root = &cobra.Command{
		Use:   "loomd",
		Short: "Loom Daemon - AI Runtime Daemon",
		Long: `Loom Daemon manages AI runtime sessions and communicates with the Loom Backend.

The daemon runs locally and is responsible for:
- Managing AI runtime sessions (Claude Code, OpenCode)
- Executing tools (Bash, Read, Write, Edit, Glob, Grep)
- Communicating with the Loom Backend via WebSocket
- Providing a local HTTP control interface`,
		Version: Version,
	}

	verbose bool
	cfgFile string
)

// Execute executes the root command
func Execute() error {
	return Root.Execute()
}

func init() {
	Root.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Verbose output")
	Root.PersistentFlags().StringVar(&cfgFile, "config", "", "Config file path")

	// 添加子命令
	Root.AddCommand(startCmd)
	Root.AddCommand(stopCmd)
	Root.AddCommand(statusCmd)
	Root.AddCommand(versionCmd)
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show version",
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Printf("Loom Daemon v%s\n", Version)
	},
}
