package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var landCmd = &cobra.Command{
	Use:   "land [commit]",
	Short: "Land a diff",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		commit := args[0]

		fmt.Printf("Landing commit: %s", commit)
	},
}

func init() {
	rootCmd.AddCommand(landCmd)
}
