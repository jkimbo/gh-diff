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

		// Make sure that diff is not dependant on another diff that hasn't landed
		// yet
		if diff.StackedOn != "" {
			fmt.Printf("Diff is stacked on %s and so can't be landed", diff.StackedOn)
			os.Exit(1)
			return
		}

		// Store current branch so that we can switch back to it later
		currentBranch, err := runCommand(
			"Get current branch",
			exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD"),
			true,
		)
		if err != nil {
			log.Fatalf("err: %v", err)
		}

		// Checkout temporary branch
		runCommand(
			"Delete branch locally",
			exec.Command(
				"git", "branch", "-D", "tmp-main",
			),
			true,
		)

		baseRef := fmt.Sprintf("origin/%s", config.DefaultBranch)

		_, err = runCommand(
			"Create new branch",
			exec.Command(
				"git", "branch", "tmp-main", baseRef,
			),
			true,
		)
		if err != nil {
			log.Fatalf("err: %v", err)
		}

		_, err = runCommand(
			"Switch to tmp-main",
			exec.Command(
				"git", "switch", "tmp-main",
			),
			true,
		)
		if err != nil {
			log.Fatalf("err: %v", err)
		}

		// Merge branch
		_, err = runCommand(
			"Merge branch",
			exec.Command(
				"git", "merge", "--squash", diff.Branch,
			),
			true,
		)
		if err != nil {
			log.Fatalf("err: %v", err)
		}

		// Push branch
		_, err = runCommand(
			"Push changes",
			exec.Command(
				"git", "push", "origin", fmt.Sprintf("tmp-main:%s", config.DefaultBranch),
			),
			true,
		)
		if err != nil {
			log.Fatalf("err: %v", err)
		}

		_, err = runCommand(
			"Switch back to current branch",
			exec.Command(
				"git", "switch", currentBranch,
			),
			true,
		)
		if err != nil {
			log.Fatalf("err: %v", err)
		}

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

	},
}

func init() {
	rootCmd.AddCommand(landCmd)
}
