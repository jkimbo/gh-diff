package cmd

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"

	"github.com/jkimbo/stacked/internal/client"
	"github.com/jkimbo/stacked/internal/diff"
	"github.com/jkimbo/stacked/utils"
	"github.com/spf13/cobra"
)

var landCmd = &cobra.Command{
	Use:   "land [commit]",
	Short: "Land a diff",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		ctx := context.Background()
		commit := args[0]

		c, err := client.NewStackedClient(ctx)
		if err != nil {
			log.Fatalf("err: %v\n", err)
		}

		diff, err := diff.NewDiffFromCommit(ctx, c, commit)
		if err != nil {
			log.Fatalf("err: %v", err)
		}

		isMerged, err := diff.IsMerged()
		if err != nil {
			log.Fatalf("err: %v", err)
		}
		if isMerged == true {
			log.Fatalf("commit already landed")
		}

		// Make sure that diff is not dependant on another diff that hasn't landed
		// yet
		stackedOnDiff, err := diff.StackedOn(ctx)
		if err != nil {
			log.Fatalf("error: %v", err)
		}
		if stackedOnDiff != nil {
			isMerged, err := stackedOnDiff.IsMerged()
			if err != nil {
				log.Fatalf("error: %v", err)
			}
			if !isMerged {
				fmt.Printf("Diff is stacked on %s that hasn't landed yet\n", diff.DBInstance.StackedOn)
				os.Exit(1)
			}
		}

		if diff.DBInstance.PRNumber == "" {
			fmt.Printf("Diff %s doesn't have a PR number\n", diff.ID)
			os.Exit(1)
			return
		}

		fmt.Printf("Landing commit: %s", commit)

		// Merge PR
		_, _, err = utils.RunGHCommand(
			"Merge PR",
			[]string{
				"pr", "merge", diff.DBInstance.PRNumber, "--squash",
			},
		)

		_, err = utils.RunCommand(
			"Update main branch",
			exec.Command(
				"git", "pull", "origin", c.DefaultBranch(), "--rebase",
			),
			true,
			false,
		)
		if err != nil {
			log.Fatalf("err: %v", err)
		}

		// TODO
		// childDiff, err := sqlDB.GetChildDiff(ctx, diff.ID)
		// if err != nil {
		// 	if err != sql.ErrNoRows {
		// 		log.Fatalf("error: %v", err)
		// 	}
		// } else {
		// 	fmt.Println("Syncing child diffs")
		// 	childDiffID := childDiff.ID

		// 	// TODO sync all stacked on diffs
		// 	for childDiffID != "" {
		// 		// TODO Update PR base branch
		// 		// TODO Sync diff
		// 	}
		// 	fmt.Println("Syncing done")
		// }
	},
}

func init() {
	rootCmd.AddCommand(landCmd)
}
