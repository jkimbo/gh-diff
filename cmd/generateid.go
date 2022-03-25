package cmd

import (
	"fmt"

	"github.com/jkimbo/stacked/utils"
	"github.com/spf13/cobra"
)

var generateIDCmd = &cobra.Command{
	Use:   "generate-id",
	Short: "Generate a diff ID",
	Run: func(cmd *cobra.Command, args []string) {
		diffID := fmt.Sprintf("d%s", utils.RandomString(5))

		fmt.Printf("%s", diffID)
	},
}

func init() {
	rootCmd.AddCommand(generateIDCmd)
}
