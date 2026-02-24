package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Long:  `Print the version, commit, and build date of secretsync.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("secretsync %s\n", Version)
		fmt.Printf("  commit: %s\n", Commit)
		fmt.Printf("  built:  %s\n", Date)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
