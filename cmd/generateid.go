package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/jkimbo/stacked/util"
)

var generateIDCmd = &cobra.Command{
	Use:   "generate-id",
	Short: "Generate a diff ID",
	Run: func(cmd *cobra.Command, args []string) {
		diffID := fmt.Sprintf("d%s", util.RandomString(5))

		fmt.Printf("%s", diffID)
	},
}

func init() {
	rootCmd.AddCommand(generateIDCmd)
}
