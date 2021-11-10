package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var syncCmd = &cobra.Command{
	Use:   "sync [commit]",
	Short: "Sync a commit",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		commit := args[0]

		fmt.Printf("Syncing commit: %s", commit)
	},
}

func init() {
	rootCmd.AddCommand(syncCmd)
}
