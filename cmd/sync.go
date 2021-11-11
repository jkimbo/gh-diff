package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

var syncCmd = &cobra.Command{
	Use:   "sync [commit]",
	Short: "Sync a diff",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		commit := args[0]

		// Check that commit is valid
		_, err := runCommand(
			"Checking commit is valid",
			exec.Command("git", "cat-file", "-e", commit),
			false,
		)
		if err != nil {
			os.Exit(1)
		}

		fmt.Println("Syncing diff:", commit)

		// Find diff trailer
		trailers, err := runCommand(
			"Get commit trailers",
			exec.Command(
				"bash",
				"-c",
				fmt.Sprintf("git show -s --format=%%B %s | git interpret-trailers --parse", commit),
			),
			true,
		)
		if err != nil {
			os.Exit(1)
		}

		lines := strings.Split(trailers, "\n")
		var diffID string
		for _, line := range lines {
			kv := strings.Split(strings.TrimSpace(line), ":")
			if kv[0] == "Diff-ID" {
				diffID = strings.TrimSpace(kv[1])
				break
			}
		}

		if diffID != "" {
			fmt.Println("Found diff:", diffID)
		} else {
			fmt.Println("Commit hasn't been synced yet")

			// Add diff
		}

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
