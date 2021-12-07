package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var generateIDCmd = &cobra.Command{
	Use:   "generate-id",
	Short: "Generate a diff ID",
	Run: func(cmd *cobra.Command, args []string) {
		diffID := fmt.Sprintf("D%s", randomString(5))

		fmt.Printf("%s", diffID)
	},
}

func init() {
	rootCmd.AddCommand(generateIDCmd)
}
