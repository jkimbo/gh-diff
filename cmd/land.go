package cmd

import (
	"context"

	"github.com/jkimbo/gh-diff/diff"
	"github.com/spf13/cobra"
)

var landCmd = &cobra.Command{
	Use:   "land [commit]",
	Short: "Land a diff",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		var err error

		ctx := context.Background()
		commit := args[0]

		c := diff.NewClient()
		err = c.Setup(ctx)
		check(err)

		err = c.LandDiff(ctx, commit)
		check(err)
	},
}

func init() {
	rootCmd.AddCommand(landCmd)
}
