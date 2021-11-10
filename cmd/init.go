package cmd

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3" // so that sqlx works with sqlite
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialise stacked",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Initialise stacked")

		newpath := filepath.Join(".", ".stacked")
		err := os.MkdirAll(newpath, os.ModePerm)
		if err != nil {
			log.Fatalf("Unable to create .stacked dir")
		}

		if _, err := os.Stat(".stacked/main.db"); errors.Is(err, os.ErrNotExist) {
			// Create DB file
			log.Println("Creating main.db...")
			file, err := os.Create(filepath.Join(".stacked", "main.db"))
			if err != nil {
				log.Fatal(err.Error())
			}
			file.Close()
			log.Println("main.db created")
		}

		db, err := sqlx.Open("sqlite3", filepath.Join(".stacked", "main.db"))
		if err != nil {
			log.Fatalf("Unable to connect to database: %v\n", err)
		}

		schema := `
			CREATE TABLE IF NOT EXISTS diffs (
				id TEXT PRIMARY KEY,
				branch TEXT,
				pr_number TEXT NULL,
				stacked_on TEXT NULL
			);
			CREATE UNIQUE INDEX idx_diffs_id ON diffs (id);
		`

		// execute a query on the server
		db.MustExec(schema)

		// TODO
		// * Determine what the main branch is
		// * Disable pushing to master
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}
