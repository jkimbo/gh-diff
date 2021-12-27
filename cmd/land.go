package cmd

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/jkimbo/stacked/db"
	"github.com/spf13/cobra"
)

var landCmd = &cobra.Command{
	Use:   "land [commit]",
	Short: "Land a diff",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		ctx := context.Background()
		commit := args[0]

		// Check that commit is valid
		_, err := runCommand(
			"Checking commit is valid",
			exec.Command("git", "cat-file", "-e", commit),
			false,
		)
		if err != nil {
			log.Fatalf("err: %v", err)
			os.Exit(1)
		}

		fmt.Printf("Landing commit: %s", commit)

		// Find diff trailer
		diffID, err := diffIDFromCommit(commit)
		if err != nil {
			log.Fatalf("err: %s", err)
		}

		if diffID == "" {
			fmt.Println("Commit is missing a DiffID")
			os.Exit(1)
			return
		}

		sqlDB, err := db.NewDB(ctx, filepath.Join(".stacked", "main.db"))
		if err != nil {
			log.Fatalf("Unable to connect to database: %v\n", err)
		}

		config, err := loadConfig()
		if err != nil {
			log.Fatalf("err: %v\n", err)
		}

		diff, err := sqlDB.GetDiff(ctx, diffID)
		if err != nil {
			if err != sql.ErrNoRows {
				log.Fatalf("error: %v", err)
			}
			fmt.Println("Commit hasn't been synced yet")
			os.Exit(1)
			return
		}

		// check if diff has already been merged
		// https://git-scm.com/docs/git-cherry
		numCommits, err := runCommand(
			"Num commits yet to be applied",
			exec.Command(
				"bash",
				"-c",
				fmt.Sprintf("git cherry origin/%s %s | grep '+' | wc -l", config.DefaultBranch, commit),
			),
			true,
		)
		if err != nil {
			log.Fatalf("error: %v", err)
		}

		if numCommits == "0" {
			fmt.Println("commit has already been merged")
			os.Exit(1)
			return
		}

		// Make sure that diff is not dependant on another diff that hasn't landed
		// yet
		if diff.StackedOn != "" {
			parentDiff, err := sqlDB.GetDiff(ctx, diff.StackedOn)
			if err != nil {
				log.Fatalf("error: %v", err)
			}

			// check if diff has already been merged
			// https://git-scm.com/docs/git-cherry
			numCommits, err := runCommand(
				"Num commits yet to be applied",
				exec.Command(
					"bash",
					"-c",
					fmt.Sprintf("git cherry origin/%s %s | grep '+' | wc -l", config.DefaultBranch, parentDiff.Branch),
				),
				true,
			)
			if err != nil {
				log.Fatalf("error: %v", err)
			}

			if numCommits != "0" {
				fmt.Printf("Diff is stacked on %s that hasn't landed yet\n", diff.StackedOn)
				os.Exit(1)
				return
			}
		}

		if diff.PRNumber == "" {
			fmt.Printf("Diff %s doesn't have a PR number\n", diff.ID)
			os.Exit(1)
			return
		}

		// Merge PR
		_, _, err = runGHCommand(
			"Merge PR",
			[]string{
				"pr", "merge", diff.PRNumber, "--squash",
			},
		)

		_, err = runCommand(
			"Update main branch",
			exec.Command(
				"git", "pull", "origin", config.DefaultBranch, "--rebase",
			),
			true,
		)
		if err != nil {
			log.Fatalf("err: %v", err)
		}

		// TODO sync all stacked on diffs
	},
}

func init() {
	rootCmd.AddCommand(landCmd)
}
