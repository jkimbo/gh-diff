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

		// TODO
		// * If diff has already been synced
		//   * Update diff and push it
		//   * Find all stacked diffs and update them as well
		// * Else
		//   * Create new diff ID and tag the commit with it
		//   * Check if parent commit is a diff
		//     * Base diff on top of parent diff
		//   * Else
		//     * Base of origin/{master,main}
		//   * Push diff and create PR
		//   * If stacked on another diff then add the list of stacked diffs to the PR description
	},
}

func init() {
	rootCmd.AddCommand(syncCmd)
}
