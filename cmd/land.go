package cmd

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/jkimbo/stacked/db"
	"github.com/jkimbo/stacked/diff"
	"github.com/jkimbo/stacked/util"
	"github.com/spf13/cobra"
)

var landCmd = &cobra.Command{
	Use:   "land [commit]",
	Short: "Land a diff",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		ctx := context.Background()
		commit := args[0]

		sqlDB, err := db.NewDB(ctx, filepath.Join(".stacked", "main.db"))
		if err != nil {
			log.Fatalf("Unable to connect to database: %v\n", err)
		}

		config, err := diff.LoadConfig()
		if err != nil {
			log.Fatalf("err: %v\n", err)
		}

		diff, err := diff.LoadDiffFromCommit(ctx, sqlDB, config, commit)
		if err != nil {
			log.Fatalf("err: %v", err)
		}

		isMerged, err := diff.IsMerged(ctx)
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
			isMerged, err := stackedOnDiff.IsMerged(ctx)
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
		_, _, err = util.RunGHCommand(
			"Merge PR",
			[]string{
				"pr", "merge", diff.DBInstance.PRNumber, "--squash",
			},
		)

		_, err = util.RunCommand(
			"Update main branch",
			exec.Command(
				"git", "pull", "origin", config.DefaultBranch, "--rebase",
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
