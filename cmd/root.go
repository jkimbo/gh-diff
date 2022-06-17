package cmd

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/go-git/go-git/v5"
	"github.com/jkimbo/gh-diff/diff"
	"github.com/spf13/cobra"
)

var repo *git.Repository

func check(err error) {
	if err != nil {
		if os.Getenv("GH_DIFF_DEBUG") == "1" {
			panic(err)
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "gh-diff <commit_sha>",
	Short: "Stacked diffs 📚",
	Run: func(cmd *cobra.Command, args []string) {
		var err error
		// TODO check that init has been run

		ctx := context.Background()

		c := diff.NewClient()

		if len(args) == 0 {
			// TODO list recent diffs
		}

		commit := args[0]

		switch commit {
		case "init":
			err = c.Init(ctx)
			check(err)
		case "land":
			err = c.Setup(ctx)
			check(err)
			err = c.LandDiff(ctx, commit)
			check(err)
		default:
			err = c.Setup(ctx)
			check(err)
			err = c.SyncDiff(ctx, commit)
			check(err)
		}

		return
	},
}

// Execute run root command
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	// Check that we are inside a git repo
	currentRepo, err := git.PlainOpen(".")
	if err != nil {
		log.Fatalf("Current directory is not a git repo")
	}

	repo = currentRepo

	// TODO setup logging
}
