package cmd

import (
	"context"
	"fmt"

	"github.com/jkimbo/gh-diff/diff"
	_ "github.com/mattn/go-sqlite3" // so that sqlx works with sqlite
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialise stacked",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		var err error
		fmt.Println("Initialise gh-diff")

		ctx := context.Background()

		c := diff.NewClient()

		err = c.Init(ctx)
		check(err)
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}
