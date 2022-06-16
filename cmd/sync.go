package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/jkimbo/gh-diff/internal/client"
	"github.com/jkimbo/gh-diff/internal/diff"
	"github.com/spf13/cobra"
)

func check(err error) {
	if err != nil {
		if os.Getenv("GH_DIFF_DEBUG") == "1" {
			panic(err)
		}
		fmt.Printf("error: %s\n", err)
		os.Exit(1)
	}
}

var syncCmd = &cobra.Command{
	Use:   "sync [commit]",
	Short: "Sync a diff",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		// TODO
		// * If diff has already been synced
		//   * Update diff and push it
		//   * Find all stacked diffs and update them as well
		// * Else
		//   * Check if parent commit is a diff
		//     * Base diff on top of parent diff
		//   * Else
		//     * Base of origin/{master,main}
		//   * Push diff and create PR
		//   * If stacked on another diff then add the list of stacked diffs to the PR description

		ctx := context.Background()
		commit := args[0]

		c, err := client.NewStackedClient(ctx)
		check(err)

		d, err := diff.NewDiffFromCommit(ctx, c, commit)
		check(err)

		fmt.Printf("syncing diff: %s (%s)\n", d.GetSubject(), d.ID)

		err = d.Sync(ctx)
		check(err)

		if d.HasPR() == false {
			err = d.CreatePR(ctx)
			check(err)
		}

		// TODO sync the rest of the stack
		dependantDiffs, err := d.GetDependantDiffs(ctx)
		check(err)

		if len(dependantDiffs) > 0 {
			fmt.Printf("%d dependant diffs to sync\n", len(dependantDiffs))

			for _, dependantDiff := range dependantDiffs {
				fmt.Printf("syncing dependant diff: %s (%s)\n", dependantDiff.GetSubject(), dependantDiff.ID)
				err = dependantDiff.Sync(ctx)
				check(err)
			}
		}

		// TODO update all PR descriptions and titles in the stack

		return

		// diffID = fmt.Sprintf("D%s", randomString(5))

		// execCmd = fmt.Sprintf(
		// 	"test $(git rev-parse HEAD) = '%s^') && git commit --amend --no-edit --trailer 'Diff-ID:%s' || true",
		// 	commit,
		// 	diffID,
		// )
		// _, err := runCommand(
		// 	"Tag commit with ID",
		// 	exec.Command(
		// 		"bash",
		// 		"-c",
		// 		fmt.Sprintf("git show -s --format=%%B %s | git interpret-trailers --parse", commit),
		// 	),
		// 	true,
		// )
		// if err != nil {
		// 	os.Exit(1)
		// }

		// Add diff
		// git rebase '5094f7d7af2a438b79a0420ccde237f161e9cbcd^' --exec 'test $(git rev-parse HEAD) = "5094f7d7af2a438b79a0420ccde237f161e9cbcd" && echo "hi" || true'
		// git commit --amend --no-edit --trailer "Test-Diff-ID:D1234"

	},
}

func init() {
	rootCmd.AddCommand(syncCmd)
}
