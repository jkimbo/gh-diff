package cmd

import (
	"fmt"
	"log"
	"os"

	"github.com/go-git/go-git/v5"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "stacked",
	Short: "stacked diffs",
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
	_, err := git.PlainOpen(".")
	if err != nil {
		log.Fatalf("Current directory is not a git repo")
	}
}
